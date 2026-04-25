package librtmp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/GGmpeg/libflv"
	"github.com/SmartBrave/GGmpeg/librtsp"
)

// rtspSession owns one RTSP TCP connection. RTSP is stateful — the
// client walks OPTIONS → DESCRIBE → SETUP[, SETUP] → PLAY → TEARDOWN
// — so we hold per-connection state for the negotiated tracks, the
// session id, and the per-track RTP packers/interleave channels.
//
// All transport is RTP-over-RTSP TCP interleaved (RFC 2326 §10.12).
// We don't open UDP sockets, which sidesteps NAT-traversal and
// channel-pair allocation.
type rtspSession struct {
	conn       net.Conn
	br         *bufio.Reader
	server     *server
	sessionID  string
	app        string
	streamID   string
	playing    int32 //atomic — 0 idle, 1 playing
	wmu        sync.Mutex //serialise writes to conn (PLAY goroutine + replies)

	// Negotiated tracks. Each track records the lower interleave
	// channel (RTP); RTCP would go on +1. We don't actually emit RTCP
	// in this minimal impl, but we still allocate the odd channel so
	// VLC/ffplay don't object.
	videoChan int    //-1 = not set up
	audioChan int
	videoSSRC uint32
	audioSSRC uint32
	videoRTP  *librtsp.RTPPacker
	audioRTP  *librtsp.RTPPacker
}

func newRTSPSession(conn net.Conn, srv *server) *rtspSession {
	return &rtspSession{
		conn:      conn,
		br:        bufio.NewReader(conn),
		server:    srv,
		videoChan: -1,
		audioChan: -1,
		//SSRCs are arbitrary 32-bit ids; pinning to a function of
		//the connection's local time gives unique-per-session values
		//that stay stable for the lifetime of the connection.
		videoSSRC: uint32(time.Now().UnixNano()) ^ 0xCAFEBABE,
		audioSSRC: uint32(time.Now().UnixNano()) ^ 0x12345678,
	}
}

// run is the per-connection event loop: parse a request, dispatch it,
// flush a response. Runs until either side closes the TCP connection
// or PLAY's goroutine reports a write failure.
func (s *rtspSession) run() {
	defer s.conn.Close()
	for {
		req, err := librtsp.ReadRequest(s.br)
		if err != nil {
			if err != io.EOF {
				fmt.Println("rtsp: read request:", err)
			}
			return
		}
		resp := s.dispatch(req)
		if resp == nil {
			return
		}
		s.wmu.Lock()
		_, werr := s.conn.Write(resp.Bytes())
		s.wmu.Unlock()
		if werr != nil {
			return
		}
	}
}

// dispatch routes a request to the per-method handler. Each handler
// returns a fully-formed response or nil to terminate the session.
func (s *rtspSession) dispatch(req *librtsp.Request) *librtsp.Response {
	resp := &librtsp.Response{
		StatusCode: 200,
		Reason:     "OK",
		Headers:    librtsp.Headers{},
	}
	if cseq := req.Headers.Get("cseq"); cseq != "" {
		resp.Headers.Set("CSeq", cseq)
	}

	switch strings.ToUpper(req.Method) {
	case "OPTIONS":
		resp.Headers.Set("Public", "OPTIONS, DESCRIBE, SETUP, PLAY, TEARDOWN, GET_PARAMETER")
	case "DESCRIBE":
		return s.handleDescribe(req, resp)
	case "SETUP":
		return s.handleSetup(req, resp)
	case "PLAY":
		return s.handlePlay(req, resp)
	case "TEARDOWN":
		atomic.StoreInt32(&s.playing, 0)
		resp.Headers.Set("Connection", "close")
		_ = s.writeResponse(resp)
		return nil
	case "GET_PARAMETER":
		//no-op keepalive
	default:
		resp.StatusCode = 501
		resp.Reason = "Not Implemented"
	}
	return resp
}

func (s *rtspSession) writeResponse(resp *librtsp.Response) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, err := s.conn.Write(resp.Bytes())
	return err
}

// handleDescribe builds an SDP for the requested room. Refuses with
// 503 (Service Unavailable) until both AVC and AAC sequence headers
// have been observed — without them the SDP would be unplayable.
func (s *rtspSession) handleDescribe(req *librtsp.Request, resp *librtsp.Response) *librtsp.Response {
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
	rm := a.Load(room)
	if rm == nil {
		resp.StatusCode = 404
		resp.Reason = "Not Found"
		return resp
	}
	s.app, s.streamID = app, room

	sdp := buildSDPFromRoom(rm)
	if sdp == nil {
		resp.StatusCode = 503
		resp.Reason = "Service Unavailable"
		resp.Headers.Set("Retry-After", "1")
		return resp
	}
	resp.Headers.Set("Content-Type", "application/sdp")
	resp.Headers.Set("Content-Base", req.URL+"/")
	resp.Body = sdp
	return resp
}

// handleSetup remembers which interleave channels the client requested
// for each track. RTSP requires a Session id on PLAY so we manufacture
// one on the first SETUP and reuse it.
func (s *rtspSession) handleSetup(req *librtsp.Request, resp *librtsp.Response) *librtsp.Response {
	transport := req.Headers.Get("transport")
	if !strings.Contains(transport, "RTP/AVP/TCP") || !strings.Contains(transport, "interleaved") {
		resp.StatusCode = 461
		resp.Reason = "Unsupported Transport"
		return resp
	}
	rtpCh, rtcpCh, ok := parseInterleavedChannels(transport)
	if !ok {
		resp.StatusCode = 400
		resp.Reason = "Bad Request"
		return resp
	}

	track := strings.ToLower(strings.TrimRight(req.URL, "/"))
	switch {
	case strings.HasSuffix(track, "/trackid=0"):
		s.videoChan = rtpCh
		s.videoRTP = librtsp.NewRTPPacker(96, s.videoSSRC)
	case strings.HasSuffix(track, "/trackid=1"):
		s.audioChan = rtpCh
		s.audioRTP = librtsp.NewRTPPacker(97, s.audioSSRC)
	default:
		resp.StatusCode = 404
		resp.Reason = "Not Found"
		return resp
	}

	if s.sessionID == "" {
		s.sessionID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	resp.Headers.Set("Transport",
		fmt.Sprintf("RTP/AVP/TCP;unicast;interleaved=%d-%d", rtpCh, rtcpCh))
	resp.Headers.Set("Session", s.sessionID+";timeout=60")
	return resp
}

// handlePlay starts the per-track packetisation goroutine that pulls
// FLV tags from the room's broadcast and writes interleaved RTP frames
// into the same TCP connection used by the RTSP control channel.
func (s *rtspSession) handlePlay(req *librtsp.Request, resp *librtsp.Response) *librtsp.Response {
	if s.app == "" || s.streamID == "" {
		resp.StatusCode = 455
		resp.Reason = "Method Not Valid In This State"
		return resp
	}
	a, ok := s.server.apps[s.app]
	if !ok {
		resp.StatusCode = 404
		resp.Reason = "Not Found"
		return resp
	}
	rm := a.Load(s.streamID)
	if rm == nil {
		resp.StatusCode = 404
		resp.Reason = "Not Found"
		return resp
	}
	resp.Headers.Set("Session", s.sessionID)
	resp.Headers.Set("Range", "npt=0.000-")

	if !atomic.CompareAndSwapInt32(&s.playing, 0, 1) {
		return resp
	}
	go s.streamLoop(rm)
	return resp
}

// streamLoop reads from the room's GOP broadcast and emits RTP packets
// for each video/audio FLV tag. Runs until the publisher disconnects
// or the TCP connection's write side fails.
func (s *rtspSession) streamLoop(room *Room) {
	defer atomic.StoreInt32(&s.playing, 0)

	//Backfill cached sequence headers as in-band data so any client
	//that decodes off the wire rather than the SDP fmtp gets the
	//SPS/PPS too.
	_, videoHdr, _ := room.snapshotHeaders()
	if s.videoChan >= 0 && videoHdr != nil {
		s.sendVideoSeqHeader(videoHdr)
	}

	gopReader := broadcast.NewBroadcastReader(room.GOP)
	for atomic.LoadInt32(&s.playing) == 1 {
		p, alive := gopReader.Read()
		if !alive {
			return
		}
		tag, ok := p.(libflv.Tag)
		if !ok {
			continue
		}
		switch t := tag.(type) {
		case *libflv.VideoTag:
			if s.videoChan < 0 {
				continue
			}
			if err := s.sendVideo(t); err != nil {
				return
			}
		case *libflv.AudioTag:
			if s.audioChan < 0 {
				continue
			}
			if err := s.sendAudio(t); err != nil {
				return
			}
		}
	}
}

// sendVideoSeqHeader emits the SPS/PPS NALs as separate single-NAL RTP
// packets at timestamp 0 so receivers that joined mid-GOP can prime
// their decoders.
func (s *rtspSession) sendVideoSeqHeader(tag *libflv.VideoTag) {
	if tag.AVCPacketType != libflv.AVC_SEQUENCE_HEADER {
		return
	}
	sps, pps, _, _, err := parseAVCDCRForRTSP(tag.Data())
	if err != nil || len(sps) == 0 {
		return
	}
	for _, nal := range [][]byte{sps, pps} {
		for _, payload := range librtsp.PackNAL(nal, librtsp.DefaultMTU) {
			pkt := s.videoRTP.Pack(false, 0, payload)
			if err := s.writeInterleaved(uint8(s.videoChan), pkt); err != nil {
				return
			}
		}
	}
}

func (s *rtspSession) sendVideo(tag *libflv.VideoTag) error {
	if tag.CodecID != libflv.FLV_VIDEO_AVC {
		return nil
	}
	if tag.AVCPacketType != libflv.AVC_NALU {
		return nil
	}
	//FLV carries H.264 in AVCC: 4-byte length prefixes. Split into
	//individual NALs and packetise each.
	nals := librtsp.SplitAVCC(tag.Data())
	if len(nals) == 0 {
		return nil
	}
	//PTS in 90 kHz clock; FLV gives us the DTS in ms, plus a
	//composition-time offset (also in ms) for B-frames.
	pts := (uint32(tag.GetTagInfo().TimeStamp) + uint32(tag.Cts)) * 90

	//Emit each NAL; the M-bit goes only on the very last packet of
	//the access unit (per RFC 6184 §5.1).
	for ni, nal := range nals {
		pieces := librtsp.PackNAL(nal, librtsp.DefaultMTU)
		for pi, piece := range pieces {
			marker := ni == len(nals)-1 && pi == len(pieces)-1
			pkt := s.videoRTP.Pack(marker, pts, piece)
			if err := s.writeInterleaved(uint8(s.videoChan), pkt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *rtspSession) sendAudio(tag *libflv.AudioTag) error {
	if tag.SoundFormat != libflv.FLV_AUDIO_AAC {
		return nil
	}
	if tag.AACPacketType != libflv.AAC_RAW {
		return nil
	}
	//AAC RTP timestamp uses sample-rate clock. Sequence header gives
	//us the rate; default to 44100 if absent.
	rate := 44100
	if tag.SoundRate < 4 {
		switch tag.SoundRate {
		case 0:
			rate = 5500
		case 1:
			rate = 11025
		case 2:
			rate = 22050
		case 3:
			rate = 44100
		}
	}
	ts := uint32(uint64(tag.GetTagInfo().TimeStamp) * uint64(rate) / 1000)
	frame := librtsp.AdtsToRaw(tag.Data())
	payload := librtsp.PackAACFrame(frame)
	pkt := s.audioRTP.Pack(true, ts, payload)
	return s.writeInterleaved(uint8(s.audioChan), pkt)
}

func (s *rtspSession) writeInterleaved(ch uint8, rtp []byte) error {
	frame := librtsp.InterleaveFrame(ch, rtp)
	s.wmu.Lock()
	defer s.wmu.Unlock()
	_, err := s.conn.Write(frame)
	return err
}

// parseRTSPURL extracts (app, streamID) from rtsp://host[:port]/app/stream
// or its trailing /trackID variants. Tolerates the "rtsp:" scheme not
// being literally present (some clients send absolute paths).
func parseRTSPURL(raw string) (string, string) {
	s := raw
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
		if slash := strings.Index(s, "/"); slash >= 0 {
			s = s[slash:]
		} else {
			return "", ""
		}
	}
	s = strings.TrimSuffix(s, "/")
	//Strip query string if any.
	if q := strings.Index(s, "?"); q >= 0 {
		s = s[:q]
	}
	parts := strings.Split(strings.TrimPrefix(s, "/"), "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// parseInterleavedChannels pulls "interleaved=N-M" out of a Transport
// header. RFC 2326 §12.39 syntax.
func parseInterleavedChannels(transport string) (rtp, rtcp int, ok bool) {
	for _, part := range strings.Split(transport, ";") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "interleaved=") {
			continue
		}
		v := strings.TrimPrefix(part, "interleaved=")
		dash := strings.Index(v, "-")
		if dash < 0 {
			rtp, err := strconv.Atoi(v)
			if err != nil {
				return 0, 0, false
			}
			return rtp, rtp + 1, true
		}
		a, err1 := strconv.Atoi(v[:dash])
		b, err2 := strconv.Atoi(v[dash+1:])
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return a, b, true
	}
	return 0, 0, false
}

// buildSDPFromRoom inspects the cached sequence headers on a Room and
// builds the SDP that DESCRIBE returns. Returns nil if the publisher
// hasn't yet emitted enough metadata.
func buildSDPFromRoom(room *Room) []byte {
	_, videoHdr, audioHdr := room.snapshotHeaders()
	if videoHdr == nil || videoHdr.AVCPacketType != libflv.AVC_SEQUENCE_HEADER {
		return nil
	}
	sps, pps, _, _, err := parseAVCDCRForRTSP(videoHdr.Data())
	if err != nil {
		return nil
	}
	p := librtsp.SDPParams{
		StreamID: room.RoomID,
		HasVideo: true,
		SPS:      sps,
		PPS:      pps,
	}
	if audioHdr != nil && audioHdr.SoundFormat == libflv.FLV_AUDIO_AAC &&
		audioHdr.AACPacketType == libflv.AAC_SEQUENCE_HEADER {
		p.HasAudio = true
		p.AudioConfig = audioHdr.Data()
		p.AudioRate = decodeAACRate(audioHdr.SoundRate)
		if audioHdr.SoundType == libflv.SND_STEREO {
			p.AudioChans = 2
		} else {
			p.AudioChans = 1
		}
	}
	return librtsp.BuildSDP(p)
}

// decodeAACRate maps the FLV 2-bit sound-rate code back to Hz. Only
// 44.1 kHz is correct for AAC per the FLV spec, but other publishers
// emit 22050/11025/5500 occasionally.
func decodeAACRate(code uint8) int {
	switch code {
	case 0:
		return 5500
	case 1:
		return 11025
	case 2:
		return 22050
	default:
		return 44100
	}
}

// parseAVCDCRForRTSP parses an AVCDecoderConfigurationRecord into its
// constituent SPS / PPS / dimensions. Re-implemented (rather than
// importing libdash) to keep librtsp / librtmp free of the libdash
// dependency cycle.
func parseAVCDCRForRTSP(src []byte) (sps, pps []byte, w, h uint16, err error) {
	if len(src) < 9 {
		err = fmt.Errorf("DCR too short: %d bytes", len(src))
		return
	}
	off := 6
	if off+2 > len(src) {
		err = fmt.Errorf("DCR truncated at SPS length")
		return
	}
	spsLen := int(src[off])<<8 | int(src[off+1])
	off += 2
	if off+spsLen > len(src) || spsLen <= 0 {
		err = fmt.Errorf("DCR truncated in SPS body")
		return
	}
	sps = append([]byte(nil), src[off:off+spsLen]...)
	off += spsLen
	if off >= len(src) {
		err = fmt.Errorf("DCR truncated before PPS count")
		return
	}
	off++ //numPPS
	if off+2 > len(src) {
		err = fmt.Errorf("DCR truncated at PPS length")
		return
	}
	ppsLen := int(src[off])<<8 | int(src[off+1])
	off += 2
	if off+ppsLen > len(src) || ppsLen <= 0 {
		err = fmt.Errorf("DCR truncated in PPS body")
		return
	}
	pps = append([]byte(nil), src[off:off+ppsLen]...)
	return
}
