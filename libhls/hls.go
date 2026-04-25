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
	streamID   string
	dir        string
	targetDur  time.Duration
	windowSize int

	// PAT/PMT template built at Start. Held read-only once populated.
	Pat *libmpeg.PAT

	// codec helpers
	Cc         map[uint16]uint8 //key:pid — reset at each segment boundary
	Ah         *libaac.AACHeader
	AvcParser  *libavc.Parser
	audioCache *audioCache
	align      *align

	// segmenter state
	mu              sync.RWMutex
	segments        []segmentInfo
	nextSeq         int
	currentFile     *os.File
	currentWriter   easyio.EasyWriter
	currentStartDTS uint64
	currentEndDTS   uint64
	lastPSI         time.Time

	// coordination
	ready       chan struct{}
	readyOnce   sync.Once
	stopped     int32 //atomic
}

func NewHls() *HLS {
	return &HLS{
		Version:    3,
		dir:        "./data",
		targetDur:  2 * time.Second,
		windowSize: 6,
		Cc:         map[uint16]uint8{},
		Ah:         &libaac.AACHeader{},
		AvcParser: &libavc.Parser{
			Pps: bytes.NewBuffer(make([]byte, libavc.MaxSpsPpsLen)),
		},
		audioCache: newAudioCache(),
		align:      &align{},
		ready:      make(chan struct{}),
	}
}

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
// first segment has been closed and added to the window.
func (hls *HLS) Playlist() []byte {
	hls.mu.RLock()
	defer hls.mu.RUnlock()
	if len(hls.segments) == 0 {
		return nil
	}
	return buildPlaylist(hls.segments)
}

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
	hls.mu.Lock()
	if hls.currentFile != nil {
		_ = hls.currentFile.Close()
		hls.currentFile = nil
	}
	segs := append([]segmentInfo(nil), hls.segments...)
	hls.segments = nil
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
	name := fmt.Sprintf("%s-%d.ts", hls.streamID, hls.nextSeq)
	fullPath := filepath.Join(hls.dir, name)
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("create segment: %w", err)
	}
	hls.currentFile = f
	hls.currentWriter = easyio.NewEasyWriter(f)
	hls.currentStartDTS = startDTS
	hls.currentEndDTS = startDTS
	//Fresh CC per segment so each segment stands alone — a mid-stream
	//joiner decoding just this segment won't see CC discontinuities.
	hls.Cc = map[uint16]uint8{}
	return hls.writePSI()
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

	seg := segmentInfo{
		filename: name,
		seq:      hls.nextSeq,
		duration: duration,
		startDTS: hls.currentStartDTS,
	}
	hls.nextSeq++

	hls.mu.Lock()
	hls.segments = append(hls.segments, seg)
	//Reap old segments outside the sliding window.
	for len(hls.segments) > hls.windowSize {
		old := hls.segments[0]
		hls.segments = hls.segments[1:]
		_ = os.Remove(filepath.Join(hls.dir, old.filename))
	}
	hls.mu.Unlock()

	hls.readyOnce.Do(func() { close(hls.ready) })
}
