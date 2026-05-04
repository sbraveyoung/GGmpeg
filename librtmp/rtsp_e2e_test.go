package librtmp

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sbraveyoung/GGmpeg/libflv"
)

// TestRTSP_E2E_OptionsDescribe stands up the RTSP session loop on a
// net.Pipe, sends OPTIONS and DESCRIBE, and asserts canonical wire
// responses. We pre-load a video sequence header into the Room so
// DESCRIBE returns 200 (not 503).
func TestRTSP_E2E_OptionsDescribe(t *testing.T) {
	srv := NewServer(":0", "live")
	app := srv.apps["live"]

	//Synthesize a Room with cached AVC sequence header so the
	//DESCRIBE handler can emit a populated SDP.
	room := NewRoom(&RTMP{server: srv, app: "live"}, "x")
	app.Store("x", room)

	sps := []byte{0x67, 0x42, 0xC0, 0x1E, 0x91, 0x40}
	pps := []byte{0x68, 0xCE, 0x06, 0xE2}
	dcr := []byte{
		0x01, 0x42, 0xC0, 0x1E,
		0xFF, 0xE1,
		byte(len(sps) >> 8), byte(len(sps)),
	}
	dcr = append(dcr, sps...)
	dcr = append(dcr, 0x01)
	dcr = append(dcr, byte(len(pps)>>8), byte(len(pps)))
	dcr = append(dcr, pps...)

	vt := &libflv.VideoTag{
		TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: 0},
		FrameType:     libflv.KEY_FRAME,
		CodecID:       libflv.FLV_VIDEO_AVC,
		AVCPacketType: libflv.AVC_SEQUENCE_HEADER,
		VideoData:     dcr,
	}
	vt.DataSize = uint32(len(vt.Data()))
	room.setVideoSequenceHeader(vt)

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	sess := newRTSPSession(a, srv)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); sess.run() }()

	//OPTIONS
	send(t, b, "OPTIONS rtsp://example/live/x RTSP/1.0\r\nCSeq: 1\r\n\r\n")
	resp := readResponse(t, b)
	if !strings.Contains(resp, "200 OK") {
		t.Errorf("OPTIONS resp = %q", resp)
	}
	if !strings.Contains(resp, "Public:") {
		t.Errorf("OPTIONS missing Public: header")
	}

	//DESCRIBE
	send(t, b, "DESCRIBE rtsp://example/live/x RTSP/1.0\r\nCSeq: 2\r\nAccept: application/sdp\r\n\r\n")
	resp = readResponse(t, b)
	if !strings.Contains(resp, "200 OK") {
		t.Fatalf("DESCRIBE expected 200; got %q", resp)
	}
	if !strings.Contains(resp, "application/sdp") {
		t.Errorf("DESCRIBE response missing Content-Type")
	}
	if !strings.Contains(resp, "m=video 0 RTP/AVP 96") {
		t.Errorf("DESCRIBE SDP missing video m-line: %q", resp)
	}

	//TEARDOWN ends the session.
	send(t, b, "TEARDOWN rtsp://example/live/x RTSP/1.0\r\nCSeq: 3\r\n\r\n")
	_ = readResponse(t, b)

	wg.Wait()
}

// TestRTSP_E2E_DescribeNoStream returns 404 when the requested
// stream isn't published yet — important so the client retry loop
// has something to anchor on.
func TestRTSP_E2E_DescribeNoStream(t *testing.T) {
	srv := NewServer(":0", "live")

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	sess := newRTSPSession(a, srv)

	go sess.run()
	send(t, b, "DESCRIBE rtsp://example/live/missing RTSP/1.0\r\nCSeq: 1\r\n\r\n")
	resp := readResponse(t, b)
	if !strings.Contains(resp, "404") {
		t.Errorf("expected 404 for unknown stream; got %q", resp)
	}
}

// send writes a CRLF-terminated RTSP request to the connection,
// failing the test on I/O error.
func send(t *testing.T, c net.Conn, req string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = c.SetWriteDeadline(deadline)
	if _, err := c.Write([]byte(req)); err != nil {
		t.Fatalf("send: %v", err)
	}
}

// readResponse reads one RTSP response (start line + headers + body
// of length Content-Length, if any) and returns the start-line +
// headers section. Body is consumed but discarded.
func readResponse(t *testing.T, c net.Conn) string {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(c)
	var sb strings.Builder
	contentLength := 0
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		sb.WriteString(line)
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "content-length:") {
			var v string
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				v = strings.TrimSpace(parts[1])
			}
			_, _ = fmtSscanf(v, &contentLength)
		}
	}
	if contentLength > 0 {
		body := make([]byte, contentLength)
		_, _ = br.Read(body)
		sb.Write(body)
	}
	return sb.String()
}

// fmtSscanf is a tiny Sscanf shim avoiding the fmt import dance —
// the test only ever parses unsigned base-10 ints out of header
// values, so a hand-rolled parser keeps the import set tight.
func fmtSscanf(s string, out *int) (int, error) {
	v := 0
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		v = v*10 + int(r-'0')
		n++
	}
	*out = v
	return n, nil
}
