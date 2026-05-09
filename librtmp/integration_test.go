package librtmp

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/SmartBrave/Athena/easyio"
)

// TestHandshake_ClientServerPipe drives both sides of the SIMPLE
// handshake over a net.Pipe and asserts both finish without error.
// This exercises the full C0/C1/C2 ↔ S0/S1/S2 sequence end-to-end.
func TestHandshake_ClientServerPipe(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	srv := &RTMP{
		readerConn:       easyio.NewEasyReader(a),
		writerConn:       easyio.NewEasyWriter(a),
		lastChunk:        map[uint32]*Chunk{},
		peerMaxChunkSize: 128,
		ownMaxChunkSize:  128,
	}
	cli := &RTMP{
		readerConn:       easyio.NewEasyReader(b),
		writerConn:       easyio.NewEasyWriter(b),
		lastChunk:        map[uint32]*Chunk{},
		peerMaxChunkSize: 128,
		ownMaxChunkSize:  128,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	srvErr := make(chan error, 1)
	cliErr := make(chan error, 1)
	go func() {
		defer wg.Done()
		srvErr <- HandshakeServer(srv)
	}()
	go func() {
		defer wg.Done()
		cliErr <- HandshakeClient(cli)
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handshake timeout")
	}
	if e := <-srvErr; e != nil {
		t.Errorf("server handshake: %v", e)
	}
	if e := <-cliErr; e != nil {
		t.Errorf("client handshake: %v", e)
	}
}

// TestParseMessage_ConnectCommand pre-loads a serialised AMF0
// `connect` command into the read buffer and asserts that ParseMessage
// successfully parses it AND that CommandMessage.Do() updates rtmp.app
// (the canonical post-connect side effect).
func TestParseMessage_ConnectCommand(t *testing.T) {
	srv := NewServer(":0", "live")
	rtmp := &RTMP{
		readerConn:       nil, //filled below
		writerConn:       nil,
		lastChunk:        map[uint32]*Chunk{},
		peerMaxChunkSize: 4096,
		ownMaxChunkSize:  4096,
		server:           srv,
	}

	//Build a single FMT0 chunk carrying the AMF0-encoded connect
	//command. The body has 3 elements: "connect", txn=1, command obj.
	body := []byte{
		0x02, 0x00, 0x07, 'c', 'o', 'n', 'n', 'e', 'c', 't',
		0x00, 0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, //txn=1
		0x03, //object marker
		0x00, 0x03, 'a', 'p', 'p',
		0x02, 0x00, 0x04, 'l', 'i', 'v', 'e',
		0x00, 0x00, 0x09,
	}
	//Construct the chunk header by hand: FMT0, csid=3 (just any
	//value ≥ 3), TS=0, length=len(body), MessageType=COMMAND_MESSAGE_AMF0,
	//StreamID=0.
	hdr := []byte{
		0x03,                                 //FMT0 + csid=3
		0x00, 0x00, 0x00,                     //timestamp = 0
		byte(len(body) >> 16), byte(len(body) >> 8), byte(len(body)),
		byte(COMMAND_MESSAGE_AMF0),
		0x00, 0x00, 0x00, 0x00, //stream_id (LE)
	}
	wire := append(hdr, body...)

	//Server-side write target — discard responses (window-ack /
	//set-chunk-size / _result) since we're only asserting parse +
	//command dispatch.
	rtmp.readerConn = easyio.NewEasyReader(bytes.NewReader(wire))
	rtmp.writerConn = easyio.NewEasyWriter(&discardWriter{})

	if err := ParseMessage(rtmp); err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if rtmp.app != "live" {
		t.Errorf("rtmp.app = %q, want \"live\" (connect Do() didn't run)", rtmp.app)
	}
}

// discardWriter swallows all bytes — used in tests that don't care
// about server-originated frames.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// TestRTMPSendCommandRaw_ChunkLayout asserts a synthesised connect
// command serialises into one FMT0 chunk under the default 4096
// chunk size — important because the receiver reassembles chunks
// before AMF parsing.
func TestRTMPSendCommandRaw_ChunkLayout(t *testing.T) {
	buf := &bytes.Buffer{}
	rtmp := &RTMP{
		writerConn:      easyio.NewEasyWriter(buf),
		ownMaxChunkSize: 4096,
	}
	rtmp.connectApp = "live"
	rtmp.tcURL = "rtmp://example/live"
	if err := rtmp.sendCommandRaw("connect", 1, map[string]interface{}{
		"app":   "live",
		"tcUrl": "rtmp://example/live",
	}, nil); err != nil {
		t.Fatalf("sendCommandRaw: %v", err)
	}
	out := buf.Bytes()
	if out[0]&0xC0 != 0x00 {
		t.Errorf("first chunk fmt = %#x, want FMT0 (top bits zero)", out[0]&0xC0)
	}
	//The body should contain the AMF0 string "connect".
	if !bytes.Contains(out, []byte("connect")) {
		t.Errorf("emitted chunk doesn't contain command name: %x", out)
	}
}
