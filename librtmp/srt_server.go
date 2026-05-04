package librtmp

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/sbraveyoung/GGmpeg/libflv"
	"github.com/sbraveyoung/GGmpeg/libsrt"
)

// srtSpec configures one SRT publish endpoint. The listener accepts
// any caller (ffmpeg, OBS, GStreamer); the first datagram lazily
// allocates a Room under apps[app]/rooms[streamID] and forwards
// transcoded FLV tags into its broadcast.
type srtSpec struct {
	address  string
	app      string
	streamID string
}

// srtBridge wraps one libsrt.Listener and the per-stream demux state
// that turns MPEG-TS into FLV tags. There's one bridge per WithSRT
// invocation.
type srtBridge struct {
	spec    srtSpec
	server  *server
	mu      sync.Mutex
	room    *Room
	demux   *libsrt.Demuxer

	// Sequence-header bookkeeping. Once SPS/PPS are extracted from
	// an in-band IDR access unit, we synthesise AVCDecoderConfigurationRecord
	// and emit the FLV video sequence header. Same idea for AAC: pull
	// the codec config out of the first ADTS frame and emit the audio
	// sequence header.
	emittedVideoSeqHdr bool
	emittedAudioSeqHdr bool
	cachedSPS          []byte
	cachedPPS          []byte
}

// startSRT installs the listener; called from server.Handler() once
// per WithSRT spec.
func startSRT(srv *server, spec srtSpec) {
	br := &srtBridge{
		spec:   spec,
		server: srv,
		demux:  libsrt.NewDemuxer(),
	}
	br.demux.OnVideo = br.onVideo
	br.demux.OnAudio = br.onAudio

	listener, err := libsrt.Listen(spec.address, spec.streamID, br.onData)
	if err != nil {
		fmt.Printf("srt listen %s: %v\n", spec.address, err)
		return
	}
	go func() {
		if err := listener.Run(); err != nil {
			fmt.Printf("srt run %s: %v\n", spec.address, err)
		}
	}()
}

// onData feeds raw TS bytes from the SRT listener into the demuxer.
// Lazy-allocates the Room on first invocation so empty publishes don't
// pollute the App's room map.
func (br *srtBridge) onData(streamID string, payload []byte) error {
	br.mu.Lock()
	if br.room == nil {
		app, ok := br.server.apps[br.spec.app]
		if !ok {
			br.mu.Unlock()
			return fmt.Errorf("srt: app %q missing", br.spec.app)
		}
		if app.Load(br.spec.streamID) != nil {
			br.mu.Unlock()
			return fmt.Errorf("srt: stream %q already publishing", br.spec.streamID)
		}
		//Pseudo-publisher RTMP struct so cleanup paths match.
		ps := &RTMP{
			peer:   "srt://" + br.spec.address,
			server: br.server,
			role:   rolePublisher,
			app:    br.spec.app,
		}
		ps.room = NewRoom(ps, br.spec.streamID)
		br.room = ps.room
		app.Store(br.spec.streamID, br.room)
	}
	br.mu.Unlock()
	br.demux.Feed(payload)
	return nil
}

// onVideo converts an AnnexB-formatted access unit into FLV video
// tag(s). On the first IDR we mine SPS/PPS, emit the AVC sequence
// header, then continue with NALU tags.
func (br *srtBridge) onVideo(au libsrt.AccessUnit) {
	nals := splitAnnexB(au.NAL)
	var sps, pps []byte
	var pictureNALs [][]byte
	for _, n := range nals {
		if len(n) == 0 {
			continue
		}
		switch n[0] & 0x1F {
		case 7:
			sps = n
		case 8:
			pps = n
		default:
			pictureNALs = append(pictureNALs, n)
		}
	}
	if !br.emittedVideoSeqHdr {
		if sps != nil {
			br.cachedSPS = sps
		}
		if pps != nil {
			br.cachedPPS = pps
		}
		if br.cachedSPS != nil && br.cachedPPS != nil {
			seq := buildAVCSequenceHeader(br.cachedSPS, br.cachedPPS)
			vt := &libflv.VideoTag{
				TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: 0},
				FrameType:     libflv.KEY_FRAME,
				CodecID:       libflv.FLV_VIDEO_AVC,
				AVCPacketType: libflv.AVC_SEQUENCE_HEADER,
				VideoData:     seq,
			}
			vt.DataSize = uint32(len(vt.Data()))
			br.room.setVideoSequenceHeader(vt)
			br.room.GOP.WriteMeta(vt)
			br.emittedVideoSeqHdr = true
		}
	}
	if len(pictureNALs) == 0 {
		return
	}
	frameType := uint8(libflv.INTER_FRAME)
	if au.Key {
		frameType = libflv.KEY_FRAME
	}
	avcc := make([]byte, 0)
	for _, n := range pictureNALs {
		var sz [4]byte
		binary.BigEndian.PutUint32(sz[:], uint32(len(n)))
		avcc = append(avcc, sz[:]...)
		avcc = append(avcc, n...)
	}
	tagTS := uint32(au.DTS / 90) //PTS/DTS in 90 kHz; FLV in ms
	cts := uint32((au.PTS - au.DTS) / 90)
	vt := &libflv.VideoTag{
		TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: tagTS},
		FrameType:     frameType,
		CodecID:       libflv.FLV_VIDEO_AVC,
		AVCPacketType: libflv.AVC_NALU,
		Cts:           cts,
		VideoData:     avcc,
	}
	vt.DataSize = uint32(len(vt.Data()))
	if au.Key {
		br.room.GOP.Reset()
	}
	br.room.GOP.Write(vt)
}

// onAudio converts an ADTS-prefixed AAC frame into one FLV audio tag.
// The first frame triggers the sequence-header emission derived from
// the ADTS profile/rate/channel fields.
func (br *srtBridge) onAudio(af libsrt.AudioFrame) {
	if len(af.Data) < 7 {
		return
	}
	if af.Data[0] != 0xFF || (af.Data[1]&0xF0) != 0xF0 {
		return //not ADTS — punt
	}
	profile := (af.Data[2] >> 6) & 0x03                              //2 bits
	sampleIndex := (af.Data[2] >> 2) & 0x0F                          //4 bits
	channels := ((af.Data[2] & 0x01) << 2) | ((af.Data[3] >> 6) & 0x03)
	if !br.emittedAudioSeqHdr {
		//AAC AudioSpecificConfig per ISO/IEC 14496-3:
		//  5 bits profile (audio object type, ADTS profile + 1)
		//  4 bits sampling frequency index
		//  4 bits channel config
		//  3 bits pad
		objType := uint8(profile) + 1
		var asc [2]byte
		asc[0] = (objType<<3)&0xF8 | (sampleIndex>>1)&0x07
		asc[1] = ((sampleIndex & 0x01) << 7) | (channels&0x0F)<<3
		soundType := uint8(libflv.SND_STEREO)
		if channels == 1 {
			soundType = libflv.SND_MONO
		}
		seq := &libflv.AudioTag{
			TagBase:       libflv.TagBase{TagType: libflv.AUDIO_TAG, TimeStamp: 0},
			SoundFormat:   libflv.FLV_AUDIO_AAC,
			SoundRate:     3,
			SoundSize:     libflv.SND_16_BIT,
			SoundType:     soundType,
			AACPacketType: libflv.AAC_SEQUENCE_HEADER,
			SoundData:     asc[:],
		}
		seq.DataSize = uint32(len(seq.Data()))
		br.room.setAudioSequenceHeader(seq)
		br.room.GOP.WriteMeta(seq)
		br.emittedAudioSeqHdr = true
	}
	soundType := uint8(libflv.SND_STEREO)
	if channels == 1 {
		soundType = libflv.SND_MONO
	}
	at := &libflv.AudioTag{
		TagBase:       libflv.TagBase{TagType: libflv.AUDIO_TAG, TimeStamp: uint32(af.PTS / 90)},
		SoundFormat:   libflv.FLV_AUDIO_AAC,
		SoundRate:     3,
		SoundSize:     libflv.SND_16_BIT,
		SoundType:     soundType,
		AACPacketType: libflv.AAC_RAW,
		SoundData:     af.Data[7:], //strip 7-byte ADTS header
	}
	at.DataSize = uint32(len(at.Data()))
	br.room.GOP.Write(at)
}

// splitAnnexB walks an AnnexB-formatted byte stream and yields one
// slice per NAL unit (start code stripped). Tolerates both 3-byte
// (0x000001) and 4-byte (0x00000001) start codes.
func splitAnnexB(buf []byte) [][]byte {
	var out [][]byte
	i := 0
	for i < len(buf) {
		//Find next start code.
		start := -1
		scLen := 0
		for j := i; j+2 < len(buf); j++ {
			if buf[j] == 0 && buf[j+1] == 0 && buf[j+2] == 1 {
				start = j
				scLen = 3
				if j > 0 && buf[j-1] == 0 {
					start = j - 1
					scLen = 4
				}
				break
			}
		}
		if start < 0 {
			break
		}
		//Find the next start after this one to bound the NAL.
		nalStart := start + scLen
		end := len(buf)
		for j := nalStart; j+2 < len(buf); j++ {
			if buf[j] == 0 && buf[j+1] == 0 && buf[j+2] == 1 {
				end = j
				if j > nalStart && buf[j-1] == 0 {
					end = j - 1
				}
				break
			}
		}
		if end > nalStart {
			out = append(out, buf[nalStart:end])
		}
		i = end
	}
	return out
}

// silence the unused-time import in case the demuxer-feed path needs
// time.Time later for stats.
var _ = time.Now
