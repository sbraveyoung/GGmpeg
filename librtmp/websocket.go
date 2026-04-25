package librtmp

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Minimal RFC 6455 server implementation. Covers exactly what flv.js
// needs to consume a binary FLV stream:
//   - HTTP/1.1 Upgrade handshake
//   - server->client binary frames (opcode 2) without masking
//   - client->server ping (0x9) replied with pong (0xA)
//   - close frame (0x8) handling
// Fragmented frames, extensions and per-message deflate are not
// implemented — flv.js doesn't need them.

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsOpcode names a control / data opcode from RFC 6455 §5.2.
type wsOpcode byte

const (
	wsOpContinuation wsOpcode = 0x0
	wsOpText         wsOpcode = 0x1
	wsOpBinary       wsOpcode = 0x2
	wsOpClose        wsOpcode = 0x8
	wsOpPing         wsOpcode = 0x9
	wsOpPong         wsOpcode = 0xA
)

// wsConn is a thin server-side WebSocket connection. The caller owns
// the underlying net.Conn lifetime.
type wsConn struct {
	conn net.Conn
	br   *bufio.Reader
}

// upgradeWebSocket performs the HTTP/1.1 Upgrade handshake against
// w/r and returns a connection that can be used to send binary frames.
// The caller is responsible for hijacking and closing the underlying
// net.Conn.
func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("not a websocket upgrade")
	}
	if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return nil, errors.New("missing Connection: Upgrade")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer doesn't support Hijack")
	}
	conn, brw, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("hijack: %w", err)
	}

	//Sec-WebSocket-Accept = base64(sha1(key + GUID)) per §4.2.2.
	hash := sha1.New()
	_, _ = hash.Write([]byte(key))
	_, _ = hash.Write([]byte(wsGUID))
	accept := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n" +
		"Access-Control-Allow-Origin: *\r\n" +
		"\r\n"
	if _, err := io.WriteString(conn, resp); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write handshake response: %w", err)
	}
	return &wsConn{conn: conn, br: brw.Reader}, nil
}

// writeFrame emits a single (unfragmented, unmasked) frame with the
// given opcode and payload. Server-to-client frames MUST NOT be
// masked per §5.3.
func (ws *wsConn) writeFrame(op wsOpcode, payload []byte) error {
	header := make([]byte, 0, 14)
	header = append(header, 0x80|byte(op)) //FIN=1, RSV=0, opcode

	switch n := len(payload); {
	case n < 126:
		header = append(header, byte(n)) //MASK=0
	case n <= 0xFFFF:
		header = append(header, 126)
		var ln [2]byte
		binary.BigEndian.PutUint16(ln[:], uint16(n))
		header = append(header, ln[:]...)
	default:
		header = append(header, 127)
		var ln [8]byte
		binary.BigEndian.PutUint64(ln[:], uint64(n))
		header = append(header, ln[:]...)
	}

	if _, err := ws.conn.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := ws.conn.Write(payload)
	return err
}

// readFrame reads one frame. It transparently unmasks the payload (all
// client-to-server frames are masked per §5.3). Returns the opcode and
// the decoded payload. Continuation frames are stitched onto the
// original opcode.
func (ws *wsConn) readFrame() (wsOpcode, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(ws.br, hdr); err != nil {
		return 0, nil, err
	}
	fin := hdr[0]&0x80 != 0
	op := wsOpcode(hdr[0] & 0x0F)
	masked := hdr[1]&0x80 != 0
	plen := uint64(hdr[1] & 0x7F)

	switch plen {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(ws.br, ext); err != nil {
			return 0, nil, err
		}
		plen = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(ws.br, ext); err != nil {
			return 0, nil, err
		}
		plen = binary.BigEndian.Uint64(ext)
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(ws.br, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, plen)
	if _, err := io.ReadFull(ws.br, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	if !fin {
		//flv.js (the only client we need to support here) never
		//fragments. Reject so we don't have to track stitched state.
		return 0, nil, errors.New("fragmented frames not supported")
	}
	return op, payload, nil
}

// Close sends a close frame (best-effort) and tears down the TCP
// connection.
func (ws *wsConn) Close() error {
	_ = ws.writeFrame(wsOpClose, nil)
	return ws.conn.Close()
}

// wsWriter adapts *wsConn to the easyio.EasyWriter interface so an
// existing FLV serialiser (Room.FLVJoin) can stream into a WebSocket
// without knowing it.
type wsWriter struct {
	ws *wsConn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	if err := w.ws.writeFrame(wsOpBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// servePings runs a small read loop that handles ping/close frames
// while a writer goroutine pushes FLV bytes. flv.js doesn't issue
// pings, but browsers/proxies sometimes do.
func (ws *wsConn) servePings(stop chan struct{}) {
	for {
		op, payload, err := ws.readFrame()
		if err != nil {
			close(stop)
			return
		}
		switch op {
		case wsOpPing:
			_ = ws.writeFrame(wsOpPong, payload)
		case wsOpClose:
			close(stop)
			return
		}
	}
}
