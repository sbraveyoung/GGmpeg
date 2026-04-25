package librtmp

import (
	"bytes"
	"net"
	"testing"

	"github.com/SmartBrave/Athena/easyio"
)

// fakeRTMP wires an in-memory pipe so chunk Send/Parse can round-trip
// without a real TCP connection.
func fakeRTMP() (*RTMP, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &RTMP{
		readerConn:       easyio.NewEasyReader(buf),
		writerConn:       easyio.NewEasyWriter(buf),
		lastChunk:        map[uint32]*Chunk{},
		peerMaxChunkSize: 4096,
		ownMaxChunkSize:  4096,
	}, buf
}

func TestChunk_FMT0_RoundTrip(t *testing.T) {
	rtmp, _ := fakeRTMP()
	payload := []byte{0xa, 0xb, 0xc, 0xd}
	out := NewChunk(SET_CHUNK_SIZE, uint32(len(payload)), 12345, FMT0, csidProtocolControl, payload)
	if err := out.Send(rtmp); err != nil {
		t.Fatalf("send: %v", err)
	}

	got, err := ParseChunk(rtmp, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.MessageTimeStamp != 12345 {
		t.Errorf("timestamp = %d, want 12345", got.MessageTimeStamp)
	}
	if got.MessageType != SET_CHUNK_SIZE {
		t.Errorf("type = %d, want %d", got.MessageType, SET_CHUNK_SIZE)
	}
	if got.MessageLength != uint32(len(payload)) {
		t.Errorf("length = %d, want %d", got.MessageLength, len(payload))
	}
	if !bytes.Equal(got.Payload, payload) {
		t.Errorf("payload = %x, want %x", got.Payload, payload)
	}
	if got.ExtendedTimestamp {
		t.Error("expected ExtendedTimestamp=false for ts < 0xFFFFFF")
	}
}

func TestChunk_ExtendedTimestamp_FMT0(t *testing.T) {
	rtmp, _ := fakeRTMP()
	const bigTS = uint32(0x01234567)
	payload := []byte{0x00, 0x00, 0x10, 0x00}
	out := NewChunk(SET_CHUNK_SIZE, uint32(len(payload)), bigTS, FMT0, csidProtocolControl, payload)
	if err := out.Send(rtmp); err != nil {
		t.Fatalf("send: %v", err)
	}
	got, err := ParseChunk(rtmp, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.MessageTimeStamp != bigTS {
		t.Errorf("timestamp = %#x, want %#x", got.MessageTimeStamp, bigTS)
	}
	if !got.ExtendedTimestamp {
		t.Error("expected ExtendedTimestamp=true for ts > 0xFFFFFF")
	}
}

func TestChunk_FMT3_Continuation(t *testing.T) {
	rtmp, _ := fakeRTMP()
	//Pretend we just received a 256-byte FMT0 chunk on csidVideo;
	//ParseChunk records it in lastChunk so the FMT3 continuation can
	//inherit fields.
	first := NewChunk(VIDEO_MESSAGE, 256, 100, FMT0, csidVideo, make([]byte, 256))
	if err := first.Send(rtmp); err != nil {
		t.Fatalf("send first: %v", err)
	}
	if _, err := ParseChunk(rtmp, nil); err != nil {
		t.Fatalf("parse first: %v", err)
	}

	//Now send and parse an FMT3 chunk continuing the same message.
	cont := &Chunk{
		ChunkBasicHeader: ChunkBasicHeader{Fmt: FMT3, CsID: csidVideo},
		ChunkMessageHeader: ChunkMessageHeader{
			MessageTimeStamp: 100,
			MessageLength:    256,
			MessageType:      VIDEO_MESSAGE,
		},
		Payload: []byte{0xde, 0xad, 0xbe, 0xef},
	}
	if err := cont.Send(rtmp); err != nil {
		t.Fatalf("send cont: %v", err)
	}
	//Continuation reads in the context of an in-flight message — pass
	//a non-nil sentinel so ParseChunk treats it as such.
	dummy := NewSetChunkSizeMessage(MessageBase{rtmp: rtmp, messageLength: 256, messagePayload: make([]byte, 252)})
	got, err := ParseChunk(rtmp, dummy)
	if err != nil {
		t.Fatalf("parse cont: %v", err)
	}
	if !bytes.Equal(got.Payload, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Errorf("payload = %x, want deadbeef", got.Payload)
	}
}

func TestParseFlvURL(t *testing.T) {
	cases := []struct {
		in        string
		wantApp   string
		wantRoom  string
		wantOK    bool
	}{
		{"/live/abc.flv", "live", "abc", true},
		{"/live/abc.flv?token=xyz", "live", "abc", true},
		{"/live/abc", "", "", false},        //no .flv
		{"/abc.flv", "", "", false},         //missing app
		{"/live/abc.flvflv", "", "", false}, //must end in exactly .flv
		{"/live//.flv", "", "", false},      //empty room (path.Clean drops //)
	}
	for _, c := range cases {
		gotApp, gotRoom, ok := parseFlvURL(c.in)
		if ok != c.wantOK || gotApp != c.wantApp || gotRoom != c.wantRoom {
			t.Errorf("parseFlvURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, gotApp, gotRoom, ok, c.wantApp, c.wantRoom, c.wantOK)
		}
	}
}

func TestParseHlsURL(t *testing.T) {
	cases := []struct {
		in       string
		wantApp  string
		wantRoom string
		wantFile string
		wantOK   bool
	}{
		{"/live/abc/index.m3u8", "live", "abc", "index.m3u8", true},
		{"/live/abc/0.ts", "live", "abc", "0.ts", true},
		{"/live/abc", "", "", "", false},
		{"/live/abc/", "", "", "", false}, //path.Clean strips trailing /
	}
	for _, c := range cases {
		gotApp, gotRoom, gotFile, ok := parseHlsURL(c.in)
		if ok != c.wantOK || gotApp != c.wantApp || gotRoom != c.wantRoom || gotFile != c.wantFile {
			t.Errorf("parseHlsURL(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				c.in, gotApp, gotRoom, gotFile, ok, c.wantApp, c.wantRoom, c.wantFile, c.wantOK)
		}
	}
}

// silence unused-import warning if tests above are pruned later.
var _ = net.IPv4
