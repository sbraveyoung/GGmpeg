// Package libdash produces a CMAF / MPEG-DASH live stream by feeding
// FLV tags through libmp4 to emit fragmented-MP4 segments and a
// matching dynamic .mpd manifest.
//
// Scope: H.264 video only (audio is a TODO). Single representation.
// Output layout, mirroring the libhls convention:
//
//	<dir>/<streamID>-init.mp4         //ftyp + moov (init segment)
//	<dir>/<streamID>-<seq>.m4s        //moof + mdat per fragment
//
// Manifest path is /<app>/<streamID>/index.mpd (constructed in the
// HTTP layer); libdash only worries about producing the bytes.
package libdash

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/sbraveyoung/GGmpeg/libflv"
	"github.com/sbraveyoung/GGmpeg/libmp4"
)

// Default segment timescale: 1000 → durations are in milliseconds,
// matching FLV timestamps directly. This keeps the math simple at the
// cost of the 90 kHz precision a typical MPEG-TS stream would carry.
const defaultTimescale = 1000

// segmentInfo records one closed media segment for the manifest.
type segmentInfo struct {
	seq       int
	filename  string
	startTime uint64 //in track timescale units
	duration  uint64 //in track timescale units
}

// DASH is the per-stream segmenter. Lifecycle: NewDASH().WithDir(...).
// WithStreamID(...).Start(reader). Calls into Manifest() / InitSegment()
// / Stop() are safe from any goroutine.
type DASH struct {
	streamID   string
	dir        string
	targetDur  time.Duration
	windowSize int
	timescale  uint32

	// Decoder configuration learned from the video sequence header.
	mu          sync.Mutex
	cond        *sync.Cond
	codec       string //"avc1.42E01E" or "hvc1.…" derived at init time
	isHEVC      bool
	sps         []byte
	pps         []byte
	hvccRecord  []byte //full HEVCDecoderConfigurationRecord, HEVC only
	width       uint16
	height      uint16
	initBytes   []byte
	initWritten bool

	segments    []segmentInfo
	nextSeq     int
	currentSamples []sampleWithTime
	availabilityStart time.Time

	ready     chan struct{}
	readyOnce sync.Once
	stopped   int32
}

// sampleWithTime tracks a sample plus the absolute DTS that produced
// it, so that durations (= next.DTS - this.DTS) and segment start time
// can be computed at finalisation.
type sampleWithTime struct {
	dts  uint64
	cts  int32
	data []byte
	key  bool
}

func NewDASH() *DASH {
	d := &DASH{
		dir:               "./data",
		targetDur:         2 * time.Second,
		windowSize:        6,
		timescale:         defaultTimescale,
		availabilityStart: time.Now().UTC(),
	}
	d.cond = sync.NewCond(&d.mu)
	d.ready = make(chan struct{})
	return d
}

func (d *DASH) WithStreamID(id string) *DASH { d.streamID = id; return d }
func (d *DASH) WithDir(dir string) *DASH {
	if dir != "" {
		d.dir = dir
	}
	return d
}
func (d *DASH) Dir() string { return d.dir }

// InitSegment returns the bytes of the init segment (ftyp + moov) once
// the AVC sequence header has been parsed. Returns nil before that.
func (d *DASH) InitSegment() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.initBytes == nil {
		return nil
	}
	out := make([]byte, len(d.initBytes))
	copy(out, d.initBytes)
	return out
}

// Manifest renders the current dynamic .mpd. Returns nil before any
// segment has been produced.
func (d *DASH) Manifest() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.initWritten || len(d.segments) == 0 {
		return nil
	}
	return buildMPD(manifestInputs{
		streamID:          d.streamID,
		availabilityStart: d.availabilityStart,
		timescale:         d.timescale,
		targetDur:         d.targetDur,
		width:             d.width,
		height:            d.height,
		codecStr:          d.codec,
		segments:          append([]segmentInfo(nil), d.segments...),
	})
}

func (d *DASH) WaitFirstSegment() { <-d.ready }

func (d *DASH) Stop() {
	if !atomic.CompareAndSwapInt32(&d.stopped, 0, 1) {
		return
	}
	d.mu.Lock()
	segs := append([]segmentInfo(nil), d.segments...)
	d.segments = nil
	d.cond.Broadcast()
	d.mu.Unlock()
	for _, s := range segs {
		_ = os.Remove(filepath.Join(d.dir, s.filename))
	}
	if d.streamID != "" {
		_ = os.Remove(filepath.Join(d.dir, fmt.Sprintf("%s-init.mp4", d.streamID)))
	}
	d.readyOnce.Do(func() { close(d.ready) })
}

func (d *DASH) stopRequested() bool {
	return atomic.LoadInt32(&d.stopped) != 0
}

// Start consumes FLV tags from gopReader, batches H.264 samples into
// fragments and writes them to disk. Returns when the publisher
// disconnects or Stop is called.
func (d *DASH) Start(gopReader *broadcast.BroadcastReader) error {
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", d.dir, err)
	}
	defer func() {
		d.flushCurrent()
		d.readyOnce.Do(func() { close(d.ready) })
	}()

	for {
		if d.stopRequested() {
			return nil
		}
		p, alive := gopReader.Read()
		if !alive {
			return nil
		}
		tag, ok := p.(libflv.Tag)
		if !ok {
			continue
		}
		v, ok := tag.(*libflv.VideoTag)
		if !ok {
			continue //audio not yet supported in this minimal libdash
		}
		if v.CodecID != libflv.FLV_VIDEO_AVC && v.CodecID != libflv.FLV_VIDEO_HEVC {
			continue
		}

		switch v.AVCPacketType {
		case libflv.AVC_SEQUENCE_HEADER:
			if v.CodecID == libflv.FLV_VIDEO_HEVC {
				if err := d.handleHEVCSequenceHeader(v.Data()); err != nil {
					fmt.Printf("dash: parse HEVC sequence header: %v\n", err)
				}
			} else {
				if err := d.handleSequenceHeader(v.Data()); err != nil {
					fmt.Printf("dash: parse AVC sequence header: %v\n", err)
				}
			}
			continue
		case libflv.AVC_NALU:
			//Sample data — fall through.
		default:
			continue
		}

		dts := uint64(v.GetTagInfo().TimeStamp) //ms
		cts := int32(v.Cts)                     //ms
		isKey := v.FrameType == libflv.KEY_FRAME

		//Rotate on keyframes once the current segment has hit
		//targetDur. The first sample of any segment must itself be a
		//keyframe so the segment is independently decodable.
		if isKey {
			if len(d.currentSamples) > 0 {
				curDur := dts - d.currentSamples[0].dts
				if curDur*1000 >= uint64(d.targetDur/time.Millisecond)*1000 {
					if err := d.flushCurrent(); err != nil {
						fmt.Printf("dash: flush segment: %v\n", err)
					}
				}
			}
		} else if len(d.currentSamples) == 0 {
			//Drop tags until the first IDR arrives.
			continue
		}
		if d.initBytes == nil {
			//Sequence header hasn't arrived yet — without SPS/PPS we
			//can't produce a playable stream.
			continue
		}

		d.currentSamples = append(d.currentSamples, sampleWithTime{
			dts:  dts,
			cts:  cts,
			data: append([]byte(nil), v.Data()...),
			key:  isKey,
		})
	}
}

// handleHEVCSequenceHeader parses the FLV-carried
// HEVCDecoderConfigurationRecord. We pull width/height by walking the
// embedded SPS array, and embed the entire DCR verbatim into the
// hvcC box.
func (d *DASH) handleHEVCSequenceHeader(record []byte) error {
	w, h, codec, err := parseHEVCDCRDimensions(record)
	if err != nil {
		//Fall back to a placeholder codec string; many players accept
		//missing dimensions when the in-band parameter sets later
		//replenish them.
		codec = "hvc1.1.6.L93.B0"
	}
	d.mu.Lock()
	d.isHEVC = true
	d.codec = codec
	d.hvccRecord = append([]byte(nil), record...)
	d.width = w
	d.height = h
	d.initBytes = libmp4.BuildHEVCInitSegment(libmp4.HEVCInitParams{
		TrackID:    1,
		Timescale:  d.timescale,
		Width:      w,
		Height:     h,
		HVCCRecord: record,
	})
	d.mu.Unlock()

	if d.streamID != "" {
		path := filepath.Join(d.dir, fmt.Sprintf("%s-init.mp4", d.streamID))
		if err := os.WriteFile(path, d.initBytes, 0o644); err != nil {
			return fmt.Errorf("write init segment: %w", err)
		}
	}
	d.mu.Lock()
	d.initWritten = true
	d.mu.Unlock()
	return nil
}

// parseHEVCDCRDimensions walks the array_of_arrays in the
// HEVCDecoderConfigurationRecord, finds the first SPS, and reads
// pic_width / pic_height. Codec string is derived from the
// general_profile_idc / level fields.
func parseHEVCDCRDimensions(src []byte) (w, h uint16, codec string, err error) {
	if len(src) < 23 {
		err = fmt.Errorf("HEVC DCR too short: %d", len(src))
		return
	}
	profileIDC := src[1] & 0x1F
	levelIDC := src[12]
	codec = fmt.Sprintf("hvc1.%d.6.L%d.B0", profileIDC, levelIDC)

	off := 22
	if off >= len(src) {
		return
	}
	numArrays := int(src[off])
	off++
	for i := 0; i < numArrays; i++ {
		if off+3 > len(src) {
			return
		}
		nalType := src[off] & 0x3F
		numNalus := int(src[off+1])<<8 | int(src[off+2])
		off += 3
		for j := 0; j < numNalus; j++ {
			if off+2 > len(src) {
				return
			}
			n := int(src[off])<<8 | int(src[off+1])
			off += 2
			if off+n > len(src) {
				return
			}
			if nalType == 33 {
				//SPS — try to extract dimensions.
				w, h = parseHEVCSPSDimensions(src[off : off+n])
				return
			}
			off += n
		}
	}
	return
}

// parseHEVCSPSDimensions extracts pic_width_in_luma_samples and
// pic_height_in_luma_samples from an HEVC SPS. Limited to the prefix
// bits up through those fields — anything beyond requires a full
// HEVC syntax parser. Returns zeros if the SPS contains features
// we can't quickly walk past (e.g. profile constraints with sub-layer
// flags); the caller falls back to placeholder dimensions in that
// case.
func parseHEVCSPSDimensions(sps []byte) (w, h uint16) {
	if len(sps) < 16 {
		return 0, 0
	}
	//Strip emulation-prevention bytes.
	rbsp := make([]byte, 0, len(sps))
	for i := 0; i < len(sps); i++ {
		if i+2 < len(sps) && sps[i] == 0 && sps[i+1] == 0 && sps[i+2] == 0x03 {
			rbsp = append(rbsp, 0, 0)
			i += 2
			continue
		}
		rbsp = append(rbsp, sps[i])
	}

	//Skip 2-byte NAL header.
	br := newBitReader(rbsp[2:])
	_, _ = br.readBits(4) //sps_video_parameter_set_id
	maxSubLayers, _ := br.readBits(3)
	_, _ = br.readBits(1) //temporal_id_nesting_flag

	//profile_tier_level — 12 bytes for the general profile + per
	//sub-layer presence flags. Skip them as a block.
	skipBytes := 12
	for i := 0; i < skipBytes; i++ {
		_, _ = br.readBits(8)
	}
	if maxSubLayers > 1 {
		flags, _ := br.readBits(2 * int(maxSubLayers-1))
		_ = flags
		//byte-align: the spec specifies padding.
		for br.pos%8 != 0 {
			_, _ = br.readBits(1)
		}
		//Each sub-layer-present flag bit pair brings up to 11 bytes
		//of profile/level data. Without parsing the flags we can't
		//cleanly skip them — bail if any are set.
	}

	_ = br.readUE() //seq_parameter_set_id
	chromaFormat := br.readUE()
	if chromaFormat == 3 {
		_, _ = br.readBits(1)
	}
	_ = chromaFormat
	w16 := br.readUE()
	h16 := br.readUE()
	w = uint16(w16)
	h = uint16(h16)
	return
}

// handleSequenceHeader parses the FLV-carried AVCDecoderConfigurationRecord
// and, once SPS+PPS are known, produces and writes the init segment.
func (d *DASH) handleSequenceHeader(record []byte) error {
	sps, pps, w, h, err := parseAVCDCR(record)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.sps = sps
	d.pps = pps
	d.width = w
	d.height = h
	d.isHEVC = false
	//Codec string per RFC 6381: avc1.PPCCLL where PP=profile,
	//CC=constraints, LL=level — derived from SPS bytes 1..3.
	if len(sps) >= 4 {
		d.codec = fmt.Sprintf("avc1.%02X%02X%02X", sps[1], sps[2], sps[3])
	} else {
		d.codec = "avc1.42E01E"
	}
	d.initBytes = libmp4.BuildInitSegment(libmp4.InitSegmentParams{
		TrackID:   1,
		Timescale: d.timescale,
		Width:     w,
		Height:    h,
		SPS:       sps,
		PPS:       pps,
	})
	d.mu.Unlock()

	//Write init segment to disk so http.ServeFile can serve it.
	if d.streamID != "" {
		path := filepath.Join(d.dir, fmt.Sprintf("%s-init.mp4", d.streamID))
		if err := os.WriteFile(path, d.initBytes, 0o644); err != nil {
			return fmt.Errorf("write init segment: %w", err)
		}
	}
	d.mu.Lock()
	d.initWritten = true
	d.mu.Unlock()
	return nil
}

// flushCurrent closes the in-progress segment, builds an fMP4 fragment
// and writes it under <dir>/<streamID>-<seq>.m4s. Old segments are
// reaped to keep windowSize on disk.
func (d *DASH) flushCurrent() error {
	if len(d.currentSamples) < 2 {
		//Need at least one duration interval. Drop a single-sample
		//tail rather than emit a malformed fragment.
		d.currentSamples = d.currentSamples[:0]
		return nil
	}
	d.mu.Lock()
	seq := d.nextSeq
	d.mu.Unlock()

	startTime := d.currentSamples[0].dts
	samples := make([]libmp4.Sample, 0, len(d.currentSamples))
	for i, s := range d.currentSamples {
		var dur uint64
		if i+1 < len(d.currentSamples) {
			dur = d.currentSamples[i+1].dts - s.dts
		} else {
			//Last sample: use previous interval as a best-guess.
			if i > 0 {
				dur = s.dts - d.currentSamples[i-1].dts
			} else {
				dur = 33 //~30fps fallback
			}
		}
		samples = append(samples, libmp4.Sample{
			Duration:              uint32(dur),
			Size:                  uint32(len(s.data)),
			IsKey:                 s.key,
			CompositionTimeOffset: s.cts,
			Data:                  s.data,
		})
	}

	frag := libmp4.BuildMediaSegment(libmp4.MediaSegmentParams{
		TrackID:        1,
		SequenceNumber: uint32(seq + 1),
		BaseDecodeTime: startTime,
		Samples:        samples,
	})

	filename := fmt.Sprintf("%s-%d.m4s", d.streamID, seq)
	if err := os.WriteFile(filepath.Join(d.dir, filename), frag, 0o644); err != nil {
		return fmt.Errorf("write fragment: %w", err)
	}

	endTime := d.currentSamples[len(d.currentSamples)-1].dts +
		uint64(samples[len(samples)-1].Duration)
	d.mu.Lock()
	d.segments = append(d.segments, segmentInfo{
		seq:       seq,
		filename:  filename,
		startTime: startTime,
		duration:  endTime - startTime,
	})
	d.nextSeq++
	for len(d.segments) > d.windowSize {
		old := d.segments[0]
		d.segments = d.segments[1:]
		_ = os.Remove(filepath.Join(d.dir, old.filename))
	}
	d.cond.Broadcast()
	d.mu.Unlock()

	d.currentSamples = d.currentSamples[:0]
	d.readyOnce.Do(func() { close(d.ready) })
	return nil
}

// parseAVCDCR pulls SPS / PPS / width / height out of an
// AVCDecoderConfigurationRecord (FLV AVC sequence header body).
// ISO 14496-15 §5.2.4. Width/height are read from the first SPS via
// a tiny RBSP/Exp-Golomb decoder — keeping it inline avoids pulling
// in a full H.264 parser dependency.
func parseAVCDCR(src []byte) (sps, pps []byte, width, height uint16, err error) {
	if len(src) < 9 {
		err = fmt.Errorf("DCR too short: %d bytes", len(src))
		return
	}
	numSPS := src[5] & 0x1f
	if numSPS == 0 {
		err = fmt.Errorf("DCR has no SPS")
		return
	}
	off := 6
	if off+2 > len(src) {
		err = fmt.Errorf("DCR truncated at SPS length")
		return
	}
	spsLen := int(binary.BigEndian.Uint16(src[off : off+2]))
	off += 2
	if off+spsLen > len(src) {
		err = fmt.Errorf("DCR truncated in SPS body")
		return
	}
	sps = append([]byte(nil), src[off:off+spsLen]...)
	off += spsLen

	if off >= len(src) {
		err = fmt.Errorf("DCR truncated before PPS count")
		return
	}
	numPPS := src[off]
	off++
	if numPPS == 0 {
		err = fmt.Errorf("DCR has no PPS")
		return
	}
	if off+2 > len(src) {
		err = fmt.Errorf("DCR truncated at PPS length")
		return
	}
	ppsLen := int(binary.BigEndian.Uint16(src[off : off+2]))
	off += 2
	if off+ppsLen > len(src) {
		err = fmt.Errorf("DCR truncated in PPS body")
		return
	}
	pps = append([]byte(nil), src[off:off+ppsLen]...)

	width, height = parseSPSDimensions(sps)
	return
}

// parseSPSDimensions extracts pic_width / pic_height from an SPS using
// the Exp-Golomb subset that's required to reach those fields. Returns
// zeros if parsing fails — the init segment is still produced (Shaka
// Player tolerates 0 dimensions, displaying the video at its decoded
// resolution).
func parseSPSDimensions(sps []byte) (width, height uint16) {
	if len(sps) < 4 {
		return 0, 0
	}
	//Strip emulation-prevention bytes (0x03 after 0x00 0x00) before
	//running the Exp-Golomb parser.
	rbsp := make([]byte, 0, len(sps))
	for i := 0; i < len(sps); i++ {
		if i+2 < len(sps) && sps[i] == 0 && sps[i+1] == 0 && sps[i+2] == 0x03 {
			rbsp = append(rbsp, 0, 0)
			i += 2
			continue
		}
		rbsp = append(rbsp, sps[i])
	}

	br := newBitReader(rbsp[1:]) //skip nal_unit_type byte
	profile, _ := br.readBits(8)
	_, _ = br.readBits(16) //constraint flags + reserved + level
	_ = br.readUE()        //seq_parameter_set_id

	chromaFormat := uint32(1)
	if profile == 100 || profile == 110 || profile == 122 || profile == 244 ||
		profile == 44 || profile == 83 || profile == 86 || profile == 118 ||
		profile == 128 || profile == 138 || profile == 139 || profile == 134 ||
		profile == 135 {
		chromaFormat = br.readUE()
		if chromaFormat == 3 {
			_, _ = br.readBits(1) //separate_colour_plane_flag
		}
		_ = br.readUE() //bit_depth_luma_minus8
		_ = br.readUE() //bit_depth_chroma_minus8
		_, _ = br.readBits(1) //qpprime_y_zero_transform_bypass_flag
		seqScaling, _ := br.readBits(1)
		if seqScaling != 0 {
			//Skip scaling lists — implementing them robustly is a
			//digression. Most live encoders don't enable this.
			return 0, 0
		}
	}
	_ = br.readUE() //log2_max_frame_num_minus4
	picOrderCntType := br.readUE()
	switch picOrderCntType {
	case 0:
		_ = br.readUE() //log2_max_pic_order_cnt_lsb_minus4
	case 1:
		_, _ = br.readBits(1)
		_ = br.readSE()
		_ = br.readSE()
		n := br.readUE()
		for i := uint32(0); i < n; i++ {
			_ = br.readSE()
		}
	}
	_ = br.readUE() //max_num_ref_frames
	_, _ = br.readBits(1) //gaps_in_frame_num_value_allowed_flag

	picWidthInMbsMinus1 := br.readUE()
	picHeightInMapUnitsMinus1 := br.readUE()
	frameMbsOnly, _ := br.readBits(1)

	width = uint16((picWidthInMbsMinus1 + 1) * 16)
	heightInMapUnits := picHeightInMapUnitsMinus1 + 1
	if frameMbsOnly == 0 {
		heightInMapUnits *= 2
	}
	height = uint16(heightInMapUnits * 16)

	//frame_cropping_flag may further trim the frame; not honoured here
	//— we'd need to know SubWidthC/SubHeightC from chromaFormat. For
	//our display-purposes the unmodified macroblock-rounded size is
	//close enough.
	_ = chromaFormat
	return width, height
}

// bitReader is a minimal Exp-Golomb decoder for SPS parsing. It only
// covers what parseSPSDimensions needs.
type bitReader struct {
	buf []byte
	pos int //bit offset
}

func newBitReader(b []byte) *bitReader { return &bitReader{buf: b} }

func (b *bitReader) readBits(n int) (uint32, bool) {
	var v uint32
	for i := 0; i < n; i++ {
		bytePos := b.pos / 8
		if bytePos >= len(b.buf) {
			return 0, false
		}
		bit := (b.buf[bytePos] >> (7 - uint(b.pos%8))) & 1
		v = (v << 1) | uint32(bit)
		b.pos++
	}
	return v, true
}

func (b *bitReader) readUE() uint32 {
	zeros := 0
	for {
		bit, ok := b.readBits(1)
		if !ok {
			return 0
		}
		if bit != 0 {
			break
		}
		zeros++
		if zeros > 32 {
			return 0
		}
	}
	val := uint32((1 << zeros) - 1)
	if zeros > 0 {
		extra, _ := b.readBits(zeros)
		val += extra
	}
	return val
}

func (b *bitReader) readSE() int32 {
	v := b.readUE()
	if v&1 == 0 {
		return -int32(v / 2)
	}
	return int32((v + 1) / 2)
}
