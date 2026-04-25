package librtmp

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUpgradeWebSocket_AcceptKey verifies that the Sec-WebSocket-Accept
// returned by the handshake matches the RFC 6455 derivation: the
// concatenation of the client key and the WS GUID, SHA-1 hashed, then
// base64-encoded.
func TestUpgradeWebSocket_AcceptKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgradeWebSocket(w, r)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		_ = ws.Close()
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	const key = "dGhlIHNhbXBsZSBub25jZQ==" //the canonical RFC 6455 example
	req := "GET / HTTP/1.1\r\n" +
		"Host: " + addr + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write request: %v", err)
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != 101 {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}

	hash := sha1.New()
	_, _ = hash.Write([]byte(key))
	_, _ = hash.Write([]byte(wsGUID))
	expected := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != expected {
		t.Errorf("accept = %q, want %q", got, expected)
	}
}

// TestWriteFrame_BinaryShort encodes a small binary payload and decodes
// it manually. Server-to-client frames must NOT be masked.
func TestWriteFrame_BinaryShort(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	ws := &wsConn{conn: server, br: bufio.NewReader(server)}
	payload := []byte{0xde, 0xad, 0xbe, 0xef}

	go func() {
		_ = ws.writeFrame(wsOpBinary, payload)
	}()

	//writeFrame splits header + payload into two Write calls; with
	//net.Pipe each call hands off only that buffer. Use ReadFull for
	//each segment so we don't race the second Write.
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(client, hdr); err != nil {
		t.Fatalf("read header: %v", err)
	}
	if hdr[0] != 0x82 { //FIN=1 | opcode=2
		t.Errorf("byte 0 = %#x, want 0x82", hdr[0])
	}
	if hdr[1] != byte(len(payload)) { //MASK=0, payload-len=4
		t.Errorf("byte 1 = %#x, want %d (unmasked)", hdr[1], len(payload))
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(client, got); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %x, want %x", got, payload)
	}
}

// TestReadFrame_ClientMaskedRoundTrip encodes a masked client frame
// using the canonical RFC 6455 §5.7 example and verifies our reader
// unmasks it correctly.
func TestReadFrame_ClientMaskedRoundTrip(t *testing.T) {
	//"Hello" with mask key 0x37fa213d — the example from §5.7.
	frame := []byte{
		0x81, 0x85, //FIN=1 opcode=1 (text), MASK=1 len=5
		0x37, 0xfa, 0x21, 0x3d, //mask key
		0x7f, 0x9f, 0x4d, 0x51, 0x58, //masked payload
	}
	ws := &wsConn{br: bufio.NewReader(bytes.NewReader(frame))}
	op, payload, err := ws.readFrame()
	if err != nil {
		t.Fatalf("readFrame: %v", err)
	}
	if op != wsOpText {
		t.Errorf("op = %#x, want text", op)
	}
	if string(payload) != "Hello" {
		t.Errorf("payload = %q, want \"Hello\"", payload)
	}
}

// TestWriteFrame_LongPayloads exercises the 16-bit and 64-bit length
// branches of the encoder.
func TestWriteFrame_LongPayloads(t *testing.T) {
	cases := []struct {
		size    int
		want1st byte //first length byte
	}{
		{125, 125},
		{126, 126}, //triggers 16-bit extended length
		{0x10000, 127}, //triggers 64-bit extended length
	}
	for _, c := range cases {
		server, client := net.Pipe()
		ws := &wsConn{conn: server, br: bufio.NewReader(server)}
		go func() {
			_ = ws.writeFrame(wsOpBinary, make([]byte, c.size))
			_ = server.Close()
		}()
		hdr := make([]byte, 2)
		if _, err := client.Read(hdr); err != nil {
			t.Fatalf("size %d: read header: %v", c.size, err)
		}
		if hdr[1] != c.want1st {
			t.Errorf("size %d: length-byte = %d, want %d", c.size, hdr[1], c.want1st)
		}
		switch c.want1st {
		case 126:
			ext := make([]byte, 2)
			_, _ = client.Read(ext)
			if int(binary.BigEndian.Uint16(ext)) != c.size {
				t.Errorf("size %d: 16-bit ext = %d", c.size, binary.BigEndian.Uint16(ext))
			}
		case 127:
			ext := make([]byte, 8)
			_, _ = client.Read(ext)
			if int(binary.BigEndian.Uint64(ext)) != c.size {
				t.Errorf("size %d: 64-bit ext = %d", c.size, binary.BigEndian.Uint64(ext))
			}
		}
		_ = client.Close()
	}
}
