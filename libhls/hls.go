package libhls

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libaac"
	"github.com/SmartBrave/GGmpeg/libavc"
	"github.com/SmartBrave/GGmpeg/libflv"
	"github.com/SmartBrave/GGmpeg/libmpeg"
)

type HLS_MODE uint8

//rtmp->hls 转码有两种方式，懒汉式和饿汉式
// 饿汉式：流推上来后立即开始转为 hls，不管有没有人拉 hls 流。推流断掉后才停止转码
// 懒汉式：第一个人拉 hls 时才开始转码，没人拉时就停掉，即使推流还没停
//
// 如果需要做录制，需要使用饿汉式，以保证录制到全量流
// 如果只是为了 hls 实时拉流，就可以使用懒汉式，没人拉 hls 时不用转码，节省性能
const (
	NONE        HLS_MODE = iota
	IMMEDIATELY          //default
	DELAY
)

// psiInterval controls how often PAT/PMT packets are re-injected within
// a segment. ISO 13818-1 §2.4.4.9 recommends at most 100 ms so a tuner
// that starts decoding mid-segment can resync quickly.
const psiInterval = 100 * time.Millisecond

// defaultPartTargetDur is the LL-HLS partial-segment target. Apple's
// guidance is "PART-TARGET should be set to a value approximating the
// frame interval"; ~333 ms (one third of a 1-s window) is a common
// choice that balances overhead against player buffer freshness.
const defaultPartTargetDur = 333 * time.Millisecond

// HLS is the per-stream transcoder. A single publisher owns one HLS
// instance; all mutable state (CC counters, PES parsers, segment list)
// is therefore not concurrent-write, but readers (playlist fetchers)
// may race with the writer so we protect the segment list with a mutex.
//
// Single-publisher invariant: the only writer to `segments`,
// `currentFile`, `currentCC`, `audioCache`, `align`, `ah`, `avcParser`
// and `lastPSI` is Start()'s goroutine. Readers (Playlist, Dir, Stop)
// take mu.
type HLS struct {
	Version int //3

	// config — set once before Start().
	streamID      string
	dir           string
	targetDur     time.Duration
	windowSize    int
	llEnabled     bool
	partTargetDur time.Duration

	// PAT/PMT template built at Start. Held read-only once populated.
	Pat *libmpeg.PAT

	// codec helpers
	Cc         map[uint16]uint8 //key:pid — reset at each segment boundary
	Ah         *libaac.AACHeader
	AvcParser  *libavc.Parser
	audioCache *audioCache
	align      *align

	// Writer-only state (mutated only by Start's goroutine, no mutex).
	currentFile     *os.File
	currentWriter   easyio.EasyWriter
	currentStartDTS uint64
	currentEndDTS   uint64
	partStartDTS    uint64
	partStartOffset int64
	currentBytes    int64 //running offset into currentFile
	lastPSI         time.Time

	// Shared state — protected by mu / cond. mu is a plain Mutex so it
	// can satisfy sync.Cond's Locker contract (RWMutex would funnel
	// every Wait into the writer half, defeating the point).
	mu             sync.Mutex
	cond           *sync.Cond
	segments       []segmentInfo
	nextSeq        int
	currentParts   []partInfo //LL-HLS: parts of the in-progress segment
	currentSegName string     //basename of in-progress segment, "" if none

	// coordination
	ready     chan struct{}
	readyOnce sync.Once
	stopped   int32 //atomic
}

// partInfo describes one LL-HLS partial segment. URI is always the
// parent segment's filename; the player addresses the bytes via the
// BYTERANGE attribute (length@offset).
type partInfo struct {
	duration    float64
	independent bool //starts with a keyframe
	byteOffset  int64
	byteLength  int64
}

func NewHls() *HLS {
	h := &HLS{
		Version:       3,
		dir:           "./data",
		targetDur:     2 * time.Second,
		windowSize:    6,
		partTargetDur: defaultPartTargetDur,
		Cc:            map[uint16]uint8{},
		Ah:            &libaac.AACHeader{},
		AvcParser: &libavc.Parser{
			Pps: bytes.NewBuffer(make([]byte, libavc.MaxSpsPpsLen)),
		},
		audioCache: newAudioCache(),
		align:      &align{},
		ready:      make(chan struct{}),
	}
	h.cond = sync.NewCond(&h.mu)
	return h
}

// WithLowLatency enables LL-HLS partial-segment emission. Playlists
// produced by Playlist() will then advertise EXT-X-PART-INF and
// EXT-X-PART entries; clients that don't understand LL-HLS extensions
// silently ignore them.
func (hls *HLS) WithLowLatency(on bool) *HLS {
	hls.llEnabled = on
	return hls
}

// PartTargetDur reports the configured LL-HLS partial-segment target
// duration. Used by the playlist builder.
func (hls *HLS) PartTargetDur() time.Duration { return hls.partTargetDur }

// LowLatency reports whether LL-HLS extensions are enabled.
func (hls *HLS) LowLatency() bool { return hls.llEnabled }

// WithStreamID sets the stream identifier used in the segment
// filenames and served playlist.
func (hls *HLS) WithStreamID(id string) *HLS {
	hls.streamID = id
	return hls
}

// WithDir sets the directory where ts segments are written. The
// directory is created on Start if it does not exist.
func (hls *HLS) WithDir(dir string) *HLS {
	if dir != "" {
		hls.dir = dir
	}
	return hls
}

// Dir returns the segment directory (defaults to "./data").
func (hls *HLS) Dir() string { return hls.dir }

// Playlist renders the current live playlist. Returns nil until the
// first segment has been closed and added to the window. When LL-HLS
// is enabled, partial-segment metadata for the in-progress segment is
// included so blocking-reload clients see the freshest possible view.
func (hls *HLS) Playlist() []byte {
	hls.mu.Lock()
	defer hls.mu.Unlock()
	if len(hls.segments) == 0 && !hls.llEnabled {
		return nil
	}
	if !hls.llEnabled {
		return buildPlaylist(hls.segments)
	}
	return buildLLPlaylist(playlistInputs{
		segments:      hls.segments,
		nextSeq:       hls.nextSeq,
		currentParts:  append([]partInfo(nil), hls.currentParts...),
		currentName:   hls.currentSegName,
		partTargetDur: hls.partTargetDur,
	})
}

// WaitForPlaylist returns the LL-HLS playlist for which the given
// _HLS_msn / _HLS_part has become available. If wantMSN is negative
// the call returns immediately with whatever is current. Times out
// after timeout (returns the current playlist anyway, matching the
// "respond with stale playlist" guidance from Apple's spec).
func (hls *HLS) WaitForPlaylist(wantMSN int, wantPart int, timeout time.Duration) []byte {
	deadline := time.Now().Add(timeout)
	hls.mu.Lock()
	for {
		if !hls.llEnabled || wantMSN < 0 {
			break
		}
		latestMSN := hls.nextSeq - 1
		latestPart := -1
		if hls.currentSegName != "" {
			latestMSN = hls.nextSeq
			latestPart = len(hls.currentParts) - 1
		}
		if wantMSN < latestMSN || (wantMSN == latestMSN && wantPart <= latestPart) {
			break
		}
		now := time.Now()
		if !now.Before(deadline) {
			break
		}
		//Cond.Wait releases mu while parked; broadcasts come from the
		//writer goroutine after each part is recorded.
		waitCh := make(chan struct{})
		go func(d time.Duration) {
			time.Sleep(d)
			hls.mu.Lock()
			hls.cond.Broadcast()
			hls.mu.Unlock()
			close(waitCh)
		}(deadline.Sub(now))
		hls.cond.Wait()
		select {
		case <-waitCh:
		default:
		}
	}
	if len(hls.segments) == 0 && !hls.llEnabled {
		hls.mu.Unlock()
		return nil
	}
	if !hls.llEnabled {
		out := buildPlaylist(hls.segments)
		hls.mu.Unlock()
		return out
	}
	out := buildLLPlaylist(playlistInputs{
		segments:      hls.segments,
		nextSeq:       hls.nextSeq,
		currentParts:  append([]partInfo(nil), hls.currentParts...),
		currentName:   hls.currentSegName,
		partTargetDur: hls.partTargetDur,
	})
	hls.mu.Unlock()
	return out
}

// currentSegmentName is unused; kept for backward compatibility.
func currentSegmentName(_ *HLS) string { return "" }

// WaitFirstSegment blocks until the first segment is available (or
// Start has returned). Callers use it to defer a 404 on the first
// playlist fetch when running in DELAY mode.
func (hls *HLS) WaitFirstSegment() {
	<-hls.ready
}

// Stop requests graceful shutdown of the transcoder and removes any
// lingering segment files on disk. Idempotent.
func (hls *HLS) Stop() {
	if !atomic.CompareAndSwapInt32(&hls.stopped, 0, 1) {
		return
	}
	if hls.currentFile != nil {
		_ = hls.currentFile.Close()
		hls.currentFile = nil
	}
	hls.mu.Lock()
	hls.currentSegName = ""
	segs := append([]segmentInfo(nil), hls.segments...)
	hls.segments = nil
	hls.cond.Broadcast()
	hls.mu.Unlock()
	for _, s := range segs {
		_ = os.Remove(filepath.Join(hls.dir, s.filename))
	}
	//Unblock anyone waiting on WaitFirstSegment if we never produced
	//one.
	hls.readyOnce.Do(func() { close(hls.ready) })
}

func (hls *HLS) stopRequested() bool {
	return atomic.LoadInt32(&hls.stopped) != 0
}

//start to generate ts segments.
func (hls *HLS) Start(gopReader *broadcast.BroadcastReader) (err error) {
	if err := os.MkdirAll(hls.dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", hls.dir, err)
	}

	hls.Pat = &libmpeg.PAT{
		TableID:                0x00,
		SectionSyntaxIndicator: 0x01,
		SectionLength:          0x0d,
		TransportStreamID:      0x01,
		VersionNumber:          0x00,
		CurrentNextIndicator:   0x01,
		SectionNumber:          0x00,
		LastSectionNumber:      0x00,
		PMTs: map[uint16]*libmpeg.PMT{
			libmpeg.PMT_PID: {
				TableID:                0x02,
				SectionSyntaxIndicator: 0x01,
				SectionLength:          0x17,
				ProgramNumber:          0x01,
				VersionNumber:          0x00,
				CurrentNextIndicator:   0x01,
				SectionNumber:          0x00,
				LastSectionNumber:      0x00,
				PCR_PID:                libmpeg.VIDEO_PID,
				ProgramInfoLength:      0x00,
				Streams: map[uint16]*libmpeg.PES{
					libmpeg.AUDIO_PID: {
						StreamID:              0xc0,
						PacketStartCodePrefix: 0x000001,
					},
					libmpeg.VIDEO_PID: {
						StreamID:              0xe0,
						PacketStartCodePrefix: 0x000001,
					},
				},
			},
		},
	}

	defer func() {
		//On graceful exit (publisher gone or Stop called) finalise the
		//current segment so players don't lose the last few seconds.
		hls.finaliseCurrent()
		hls.readyOnce.Do(func() { close(hls.ready) })
	}()

outer:
	for {
		if hls.stopRequested() {
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

		pes, pid, videoFrameKey, skip := hls.toPES(tag)
		if skip {
			continue
		}

		//Segment rotation decision — performed on keyframes only so
		//every .ts begins with an IDR frame and is independently
		//decodable. Duration is tracked in 90 kHz PTS units because
		//that's what we already multiply the FLV timestamps by.
		if videoFrameKey {
			if hls.currentFile == nil {
				if err := hls.openSegment(pes.DTS); err != nil {
					return err
				}
			} else {
				curDur := float64(pes.DTS-hls.currentStartDTS) / 90000.0
				if curDur*float64(time.Second) >= float64(hls.targetDur) {
					if err := hls.rotate(pes.DTS); err != nil {
						return err
					}
				}
			}
		} else if hls.currentFile == nil {
			//No segment yet: drop tags until the first keyframe
			//arrives. Avoids emitting a segment that doesn't start at
			//an IDR.
			continue
		}

		//PAT/PMT periodicity: re-inject every psiInterval so clients
		//that join decoding mid-segment find a PSI quickly.
		if time.Since(hls.lastPSI) >= psiInterval {
			if err := hls.writePSI(); err != nil {
				return err
			}
		}

		firstTS := true
		for {
			finish, muxErr := libmpeg.NewTs(pid, hls.Cc, firstTS).Mux(pes, videoFrameKey && firstTS, pes.DTS, hls.currentWriter)
			if muxErr != nil {
				fmt.Printf("ts.Mux pes error:%+v\n", muxErr)
				continue outer
			}
			firstTS = false
			if finish {
				break
			}
		}
		if pes.DTS > hls.currentEndDTS {
			hls.currentEndDTS = pes.DTS
		}

		//LL-HLS partial-segment boundary check. Close the current part
		//once it has accumulated at least PART-TARGET seconds of media
		//or whenever a fresh keyframe arrived (so subsequent parts
		//align with sub-GOP boundaries when the encoder emits short
		//GOPs). Performed AFTER muxing so the just-written PES belongs
		//to the part we're closing — the next part starts at the
		//current write offset.
		if hls.llEnabled && hls.currentFile != nil {
			partDur := float64(pes.DTS-hls.partStartDTS) / 90000.0
			if partDur*float64(time.Second) >= float64(hls.partTargetDur) {
				hls.closeCurrentPart(pes.DTS, false)
			}
		}
	}
}

// toPES lifts an FLV tag into a libmpeg.PES plus routing metadata. It
// returns skip=true when the tag should be dropped (unsupported codec,
// sequence header, or audio still buffering in the ADTS cache).
func (hls *HLS) toPES(tag libflv.Tag) (pes *libmpeg.PES, pid uint16, videoKey bool, skip bool) {
	switch tag.GetTagInfo().TagType {
	case libflv.AUDIO_TAG:
		pa, _ := tag.(*libflv.AudioTag)
		switch pa.SoundFormat {
		case libflv.FLV_AUDIO_AAC:
			if pa.AACPacketType == libflv.AAC_SEQUENCE_HEADER {
				if err := hls.Ah.Parse(pa.Data()); err != nil {
					fmt.Printf("parse aac header error:%+v\n", err)
				}
				return nil, 0, false, true
			}
			pid = libmpeg.AUDIO_PID
			pes = hls.Pat.PMTs[libmpeg.PMT_PID].Streams[libmpeg.AUDIO_PID]
			pes.DTS = uint64(tag.GetTagInfo().TimeStamp * 90)
			pes.PTS_DTSFlag = 0x02
			pes.PESHeaderDataLength = 0x05
			pes.Index = 0
			pes.HeaderIndex = 0

			aacHeader := hls.Ah.Adts(pa.Data())
			pes.Data = append(aacHeader, pa.Data()...)

			rate := 44100
			if hls.Ah.SampleRate <= uint8(len(libaac.AACRates)-1) {
				rate = libaac.AACRates[hls.Ah.SampleRate]
			}
			hls.align.align(&pes.DTS, uint32(90000*1024/rate))
			pes.PTS = pes.DTS

			hls.audioCache.Cache(pes.Data, pes.PTS)
			if hls.audioCache.CacheNum() < cache_max_frames {
				return nil, 0, false, true
			}
			_, pes.PTS, pes.Data = hls.audioCache.GetFrame()
			pes.DTS = pes.PTS
			return pes, pid, false, false
		default:
			return nil, 0, false, true
		}

	case libflv.VIDEO_TAG:
		pv, _ := tag.(*libflv.VideoTag)
		switch pv.CodecID {
		case libflv.FLV_VIDEO_AVC:
			if pv.FrameType == libflv.KEY_FRAME && pv.AVCPacketType == libflv.AVC_SEQUENCE_HEADER {
				if err := hls.AvcParser.ParseSpecificInfo(pv.Data()); err != nil {
					fmt.Printf("parse avc header error:%+v\n", err)
				}
				return nil, 0, false, true
			}
			videoKey = pv.FrameType == libflv.KEY_FRAME
			pid = libmpeg.VIDEO_PID
			pes = hls.Pat.PMTs[libmpeg.PMT_PID].Streams[libmpeg.VIDEO_PID]

			pes.DTS = uint64(tag.GetTagInfo().TimeStamp * 90)
			pes.PTS = pes.DTS
			pes.PTS_DTSFlag = 0x02
			pes.PESHeaderDataLength = 0x05
			pes.Index = 0
			pes.HeaderIndex = 0

			if hls.AvcParser.IsNaluHeader(pv.Data()) {
				pes.Data = pv.Data()
			} else {
				buf := bytes.NewBuffer([]byte{})
				if err := hls.AvcParser.GetAnnexbH264(pv.Data(), easyio.NewEasyWriter(buf)); err != nil {
					fmt.Printf("AvcParser.GetAnnexbH264 error:%+v\n", err)
					return nil, 0, false, true
				}
				data, err := io.ReadAll(buf)
				if err != nil {
					fmt.Printf("io.ReadAll error:%+v\n", err)
					return nil, 0, false, true
				}
				pes.Data = data
			}
			if pv.Cts != 0 {
				pes.PTS_DTSFlag = 0x03
				pes.PTS = pes.DTS + uint64(pv.Cts*90)
			}
			return pes, pid, videoKey, false
		default:
			return nil, 0, false, true
		}
	}
	return nil, 0, false, true
}

// openSegment creates a fresh .ts file and writes an initial PAT/PMT.
func (hls *HLS) openSegment(startDTS uint64) error {
	hls.mu.Lock()
	name := fmt.Sprintf("%s-%d.ts", hls.streamID, hls.nextSeq)
	hls.mu.Unlock()
	fullPath := filepath.Join(hls.dir, name)
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create segment: %w", err)
	}
	hls.currentFile = f
	hls.currentBytes = 0
	hls.currentWriter = newCountingWriter(f, &hls.currentBytes)
	hls.currentStartDTS = startDTS
	hls.currentEndDTS = startDTS
	hls.partStartDTS = startDTS
	hls.partStartOffset = 0

	hls.mu.Lock()
	hls.currentSegName = name
	hls.currentParts = hls.currentParts[:0]
	hls.mu.Unlock()

	//Fresh CC per segment so each segment stands alone — a mid-stream
	//joiner decoding just this segment won't see CC discontinuities.
	hls.Cc = map[uint16]uint8{}
	return hls.writePSI()
}

// closeCurrentPart records the bytes written since the last part
// boundary as a partInfo on the in-progress segment, broadcasts the
// cond so blocking-reload clients can wake up, and starts a new part
// from the current write offset. independent indicates whether the
// new (next) part will start with a keyframe.
func (hls *HLS) closeCurrentPart(endDTS uint64, nextIndependent bool) {
	if hls.currentFile == nil {
		return
	}
	dur := float64(endDTS-hls.partStartDTS) / 90000.0
	if dur <= 0 {
		return
	}
	length := hls.currentBytes - hls.partStartOffset
	if length <= 0 {
		return
	}
	independent := len(hls.currentParts) == 0 //first part — starts at IDR+PSI
	hls.mu.Lock()
	hls.currentParts = append(hls.currentParts, partInfo{
		duration:    dur,
		independent: independent,
		byteOffset:  hls.partStartOffset,
		byteLength:  length,
	})
	hls.cond.Broadcast()
	hls.mu.Unlock()

	hls.partStartDTS = endDTS
	hls.partStartOffset = hls.currentBytes
	_ = nextIndependent //tracked via the "first part of new segment" rule
}

// countingWriter wraps an io.Writer and increments *n by the number of
// bytes it actually transfers. We use it to track the current segment's
// byte offset for LL-HLS partial BYTERANGE attributes.
type countingWriter struct {
	w io.Writer
	n *int64
}

func newCountingWriter(w io.Writer, n *int64) easyio.EasyWriter {
	return easyio.NewEasyWriter(&countingWriter{w: w, n: n})
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	written, err := cw.w.Write(p)
	*cw.n += int64(written)
	return written, err
}

// writePSI emits one PAT and one PMT to the current segment and stamps
// lastPSI for periodicity tracking.
func (hls *HLS) writePSI() error {
	if hls.currentWriter == nil {
		return nil
	}
	_, err1 := libmpeg.NewTs(libmpeg.PAT_PID, hls.Cc, true).Mux(hls.Pat, false, 0, hls.currentWriter)
	_, err2 := libmpeg.NewTs(libmpeg.PMT_PID, hls.Cc, true).Mux(hls.Pat.PMTs[libmpeg.PMT_PID], false, 0, hls.currentWriter)
	if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
		return err
	}
	hls.lastPSI = time.Now()
	return nil
}

// rotate closes the current segment, reaps old ones beyond the window,
// and opens a new one starting at the given DTS.
func (hls *HLS) rotate(nextStartDTS uint64) error {
	hls.finaliseCurrent()
	return hls.openSegment(nextStartDTS)
}

// finaliseCurrent closes and records the in-progress segment. Safe to
// call when no segment is open.
func (hls *HLS) finaliseCurrent() {
	if hls.currentFile == nil {
		return
	}
	//Flush any tail bytes since the last part boundary into one final
	//part, so LL-HLS clients have a complete partial-segment record
	//for this MSN.
	if hls.llEnabled && hls.currentBytes > hls.partStartOffset {
		hls.closeCurrentPart(hls.currentEndDTS, false)
	}

	name := filepath.Base(hls.currentFile.Name())
	_ = hls.currentFile.Close()
	hls.currentFile = nil
	hls.currentWriter = nil

	duration := float64(hls.currentEndDTS-hls.currentStartDTS) / 90000.0
	if duration <= 0 {
		//Never observed a second tick; assume a minimum so the
		//playlist target duration calculation doesn't underflow.
		duration = float64(hls.targetDur) / float64(time.Second)
	}

	hls.mu.Lock()
	seg := segmentInfo{
		filename: name,
		seq:      hls.nextSeq,
		duration: duration,
		startDTS: hls.currentStartDTS,
		parts:    append([]partInfo(nil), hls.currentParts...),
	}
	hls.currentParts = hls.currentParts[:0]
	hls.currentSegName = ""
	hls.nextSeq++
	hls.segments = append(hls.segments, seg)
	//Reap old segments outside the sliding window.
	for len(hls.segments) > hls.windowSize {
		old := hls.segments[0]
		hls.segments = hls.segments[1:]
		_ = os.Remove(filepath.Join(hls.dir, old.filename))
	}
	hls.cond.Broadcast()
	hls.mu.Unlock()

	hls.readyOnce.Do(func() { close(hls.ready) })
}
