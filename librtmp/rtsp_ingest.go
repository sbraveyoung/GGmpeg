package librtmp

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/sbraveyoung/GGmpeg/libflv"
	"github.com/sbraveyoung/GGmpeg/librtsp"
)

// rtspIngest carries the publisher-side state added when the client
// opts into ANNOUNCE/RECORD instead of DESCRIBE/PLAY. Stored on the
// rtspSession when the publish flow kicks off.
type rtspIngest struct {
	parsed  *librtsp.ParsedSDP
	videoCh int //RTSP interleave channel (RTP), -1 if not negotiated
	audioCh int
	videoRA *librtsp.H264Reassembler
	//Audio re-assembly is per-packet (every packet is one+ frame), so
	//no per-publisher state is needed — we just decode each RTP
	//payload via librtsp.AACAUExtract.
}

// handleAnnounce picks up an SDP body the publisher just pushed,
// extracts SPS/PPS and AAC config, and provisions a local Room so any
// downstream protocols (HTTP-FLV / HLS / DASH / RTSP PLAY) can mirror
// the stream.
func (s *rtspSession) handleAnnounce(req *librtsp.Request, resp *librtsp.Response) *librtsp.Response {
	app, room := parseRTSPURL(req.URL)
	if app == "" || room == "" {
		resp.StatusCode = 400
		resp.Reason = "Bad Request"
		return resp
	}
	a, ok := s.server.apps[app]
	if !ok {
		resp.StatusCode = 404
		resp.Reason = "Not Found"
		return resp
	}
	if a.Load(room) != nil {
		resp.StatusCode = 461
		resp.Reason = "Stream Already Publishing"
		return resp
	}

	parsed, err := librtsp.ParseSDP(req.Body)
	if err != nil {
		resp.StatusCode = 400
		resp.Reason = "Bad SDP: " + err.Error()
		return resp
	}

	//Build a synthetic RTMP "pseudo-publisher" so the room cleanup
	//path (Room.Close on TCP disconnect) works the same as for real
	//RTMP publishers.
	rtmp := &RTMP{
		peer:   s.conn.RemoteAddr().String(),
		server: s.server,
		role:   rolePublisher,
		app:    app,
	}
	rtmp.room = NewRoom(rtmp, room)
	a.Store(room, rtmp.room)

	s.app, s.streamID = app, room
	s.ingest = &rtspIngest{
		parsed:  parsed,
		videoCh: -1,
		audioCh: -1,
		videoRA: &librtsp.H264Reassembler{},
	}
	s.publishRTMP = rtmp

	//Emit the AVC + AAC sequence headers into the GOP so subscribers
	//that join before any media tag arrives still receive the decoder
	//config. We synthesize libflv tags (TimeStamp 0) that mirror what
	//an RTMP publisher would have written.
	if parsed.HasVideo && len(parsed.SPS) > 0 && len(parsed.PPS) > 0 {
		seqHdr := buildAVCSequenceHeader(parsed.SPS, parsed.PPS)
		vt := &libflv.VideoTag{
			TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: 0},
			FrameType:     libflv.KEY_FRAME,
			CodecID:       libflv.FLV_VIDEO_AVC,
			AVCPacketType: libflv.AVC_SEQUENCE_HEADER,
			VideoData:     seqHdr,
		}
		vt.DataSize = uint32(len(vt.Data()))
		rtmp.room.setVideoSequenceHeader(vt)
		rtmp.room.GOP.WriteMeta(vt)
	}
	if parsed.HasAudio && len(parsed.AudioConfig) >= 2 {
		soundRate := uint8(3) //AAC always advertises 44.1 kHz in the FLV header
		soundType := uint8(libflv.SND_STEREO)
		if parsed.AudioChans == 1 {
			soundType = libflv.SND_MONO
		}
		at := &libflv.AudioTag{
			TagBase:       libflv.TagBase{TagType: libflv.AUDIO_TAG, TimeStamp: 0},
			SoundFormat:   libflv.FLV_AUDIO_AAC,
			SoundRate:     soundRate,
			SoundSize:     libflv.SND_16_BIT,
			SoundType:     soundType,
			AACPacketType: libflv.AAC_SEQUENCE_HEADER,
			SoundData:     append([]byte(nil), parsed.AudioConfig...),
		}
		at.DataSize = uint32(len(at.Data()))
		rtmp.room.setAudioSequenceHeader(at)
		rtmp.room.GOP.WriteMeta(at)
	}
	return resp
}

// handleRecord starts the RTP-receive loop. The loop reads $-prefixed
// interleaved frames off the RTSP TCP connection until EOF, parses the
// embedded RTP packets, and forwards reassembled access units into
// the room's GOP broadcast.
func (s *rtspSession) handleRecord(req *librtsp.Request, resp *librtsp.Response) *librtsp.Response {
	if s.publishRTMP == nil || s.ingest == nil {
		resp.StatusCode = 455
		resp.Reason = "Method Not Valid In This State"
		return resp
	}
	resp.Headers.Set("Session", s.sessionID)

	//Send the 200 OK first, then enter the ingest loop. Returning nil
	//tells dispatch's caller "you take it from here" — the run() loop
	//calls writeResponse but then we hijack and read interleaved
	//frames forever.
	if err := s.writeResponse(resp); err != nil {
		return nil
	}
	atomic.StoreInt32(&s.recording, 1)
	s.runIngestLoop()
	return nil
}

// runIngestLoop reads RTP packets off whichever transport(s) the
// client negotiated in SETUP — TCP-interleaved $-frames, UDP, or both.
// UDP tracks each get their own read goroutine; the TCP control loop
// concurrently handles GET_PARAMETER / TEARDOWN / etc.
func (s *rtspSession) runIngestLoop() {
	defer atomic.StoreInt32(&s.recording, 0)
	defer func() {
		if s.publishRTMP != nil {
			s.publishRTMP.cleanup()
		}
		if s.videoUDP != nil {
			_ = s.videoUDP.Close()
		}
		if s.audioUDP != nil {
			_ = s.audioUDP.Close()
		}
		if s.videoUDPRTCP != nil {
			_ = s.videoUDPRTCP.Close()
		}
		if s.audioUDPRTCP != nil {
			_ = s.audioUDPRTCP.Close()
		}
	}()

	//Spawn a UDP reader per track that was set up in UDP mode.
	if s.videoUDP != nil {
		go s.runUDPReader(s.videoUDP, false)
	}
	if s.audioUDP != nil {
		go s.runUDPReader(s.audioUDP, true)
	}

	//Some clients (FFmpeg notably) send periodic GET_PARAMETER pings
	//interleaved with media. Detect "$" framing vs the start of an
	//RTSP keyword by peek-and-dispatch.
	for {
		first, err := s.br.ReadByte()
		if err != nil {
			return
		}
		if first != '$' {
			//Push the byte back and parse as an RTSP request.
			if err := s.br.UnreadByte(); err != nil {
				return
			}
			req, err := librtsp.ReadRequest(s.br)
			if err != nil {
				return
			}
			resp := s.dispatch(req)
			if resp == nil {
				return
			}
			if err := s.writeResponse(resp); err != nil {
				return
			}
			continue
		}
		hdr := make([]byte, 3)
		if _, err := s.br.Read(hdr); err != nil {
			return
		}
		ch := hdr[0]
		ln := int(binary.BigEndian.Uint16(hdr[1:3]))
		body := make([]byte, ln)
		if _, err := s.br.Read(body); err != nil {
			return
		}
		s.handleInterleaved(ch, body)
	}
}

// runUDPReader pumps RTP packets off a per-track UDP socket. isAudio
// distinguishes the two depacketisation paths.
func (s *rtspSession) runUDPReader(c *net.UDPConn, isAudio bool) {
	buf := make([]byte, 1500)
	for atomic.LoadInt32(&s.recording) == 1 {
		n, _, err := c.ReadFromUDP(buf)
		if err != nil {
			return
		}
		pkt, perr := librtsp.ParseRTP(append([]byte(nil), buf[:n]...))
		if perr != nil {
			continue
		}
		if isAudio {
			s.handleAudioRTP(pkt)
		} else {
			s.handleVideoRTP(pkt)
		}
	}
}

// handleInterleaved routes one decoded interleaved frame to the
// per-track depacketiser.
func (s *rtspSession) handleInterleaved(channel uint8, payload []byte) {
	pkt, err := librtsp.ParseRTP(payload)
	if err != nil {
		return
	}
	switch int(channel) {
	case s.ingest.videoCh:
		s.handleVideoRTP(pkt)
	case s.ingest.audioCh:
		s.handleAudioRTP(pkt)
	}
}

func (s *rtspSession) handleVideoRTP(pkt *librtsp.RTPPacket) {
	nals, ts := s.ingest.videoRA.Push(pkt)
	if len(nals) == 0 {
		return
	}
	avcc := librtsp.AVCCFromNALs(nals)
	frameType := uint8(libflv.INTER_FRAME)
	if librtsp.ContainsKeyframe(nals) {
		frameType = libflv.KEY_FRAME
	}
	tagTS := ts / 90 //RTP video clock is 90 kHz; FLV tags are ms
	vt := &libflv.VideoTag{
		TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: tagTS},
		FrameType:     frameType,
		CodecID:       libflv.FLV_VIDEO_AVC,
		AVCPacketType: libflv.AVC_NALU,
		VideoData:     avcc,
	}
	vt.DataSize = uint32(len(vt.Data()))
	if frameType == libflv.KEY_FRAME {
		s.publishRTMP.room.GOP.Reset()
	}
	s.publishRTMP.room.GOP.Write(vt)
}

func (s *rtspSession) handleAudioRTP(pkt *librtsp.RTPPacket) {
	frames := librtsp.AACAUExtract(pkt.Payload)
	rate := s.ingest.parsed.AudioRate
	if rate <= 0 {
		rate = 44100
	}
	for _, frame := range frames {
		tagTS := uint32(uint64(pkt.Timestamp) * 1000 / uint64(rate))
		soundType := uint8(libflv.SND_STEREO)
		if s.ingest.parsed.AudioChans == 1 {
			soundType = libflv.SND_MONO
		}
		at := &libflv.AudioTag{
			TagBase:       libflv.TagBase{TagType: libflv.AUDIO_TAG, TimeStamp: tagTS},
			SoundFormat:   libflv.FLV_AUDIO_AAC,
			SoundRate:     3,
			SoundSize:     libflv.SND_16_BIT,
			SoundType:     soundType,
			AACPacketType: libflv.AAC_RAW,
			SoundData:     frame,
		}
		at.DataSize = uint32(len(at.Data()))
		s.publishRTMP.room.GOP.Write(at)
	}
}

// buildAVCSequenceHeader packs an AVCDecoderConfigurationRecord around
// the given SPS / PPS. Mirrors what an RTMP publisher would have sent
// in its first video tag (FLV-spec AVCPacketType=0).
func buildAVCSequenceHeader(sps, pps []byte) []byte {
	if len(sps) < 4 {
		return nil
	}
	out := []byte{
		0x01,        //configurationVersion
		sps[1],      //profile
		sps[2],      //compatibility
		sps[3],      //level
		0xFF,        //6 bits reserved + lengthSizeMinusOne=3
		0xE1,        //3 bits reserved + numSPS=1
		byte(len(sps) >> 8), byte(len(sps)),
	}
	out = append(out, sps...)
	out = append(out, 0x01) //numPPS
	out = append(out, byte(len(pps)>>8), byte(len(pps)))
	out = append(out, pps...)
	return out
}

// trackPathMatches reports whether the SETUP URL terminates in the
// given track-control suffix (e.g. "trackID=0" or a relative URL).
// Tolerates leading slashes and trailing slashes.
func trackPathMatches(setupURL, control string) bool {
	if control == "" {
		return false
	}
	url := strings.TrimSuffix(setupURL, "/")
	control = strings.TrimSuffix(control, "/")
	if strings.Contains(control, "://") {
		return strings.EqualFold(url, control)
	}
	return strings.HasSuffix(strings.ToLower(url), strings.ToLower(control))
}

// notifyIngestSetup records which interleave channel a track will
// arrive on so handleInterleaved can dispatch correctly. Returns false
// when the URL doesn't match any advertised track.
func (s *rtspSession) notifyIngestSetup(setupURL string, rtpCh int) (audio bool, ok bool) {
	if s.ingest == nil {
		return false, false
	}
	if s.ingest.parsed.HasVideo && trackPathMatches(setupURL, s.ingest.parsed.VideoControl) {
		s.ingest.videoCh = rtpCh
		return false, true
	}
	if s.ingest.parsed.HasAudio && trackPathMatches(setupURL, s.ingest.parsed.AudioControl) {
		s.ingest.audioCh = rtpCh
		return true, true
	}
	return false, false
}

// silence error from importing fmt only sometimes.
var _ = fmt.Sprintf
