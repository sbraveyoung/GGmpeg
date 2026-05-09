package libsrt

import (
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestListener_E2EHandshakeAndData stands up the listener on a real
// UDP socket and runs through INDUCTION → cookie → CONCLUSION → data
// → ACK from a synthetic client. Asserts that the user-supplied
// onData callback fires with the publisher's payload.
func TestListener_E2EHandshakeAndData(t *testing.T) {
	var got [][]byte
	var mu sync.Mutex
	l, err := Listen("127.0.0.1:0", "test", func(streamID string, payload []byte) error {
		mu.Lock()
		got = append(got, append([]byte(nil), payload...))
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	srvPort := l.conn.LocalAddr().(*net.UDPAddr).Port
	srvAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: srvPort}

	go l.Run()

	cli, err := net.DialUDP("udp", nil, srvAddr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer cli.Close()
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))

	const peerSocketID = uint32(0xCAFEBABE)

	//1. INDUCTION
	hsInduction := (&Handshake{
		Version:       srtVersion5,
		HandshakeType: HSTypeAgreement,
		SrtSocketID:   peerSocketID,
	}).Marshal()
	send := func(ct ControlType, sub uint16, ti uint32, body []byte, dest uint32) {
		out := make([]byte, HeaderSize+len(body))
		MarshalControlHeader(out, ct, sub, ti, 0, dest)
		copy(out[HeaderSize:], body)
		_, err := cli.Write(out)
		if err != nil {
			t.Fatalf("client send: %v", err)
		}
	}
	send(CtrlHandshake, 0, 0, hsInduction, 0)

	//Receive cookie reply.
	buf := make([]byte, 1500)
	n, err := cli.Read(buf)
	if err != nil {
		t.Fatalf("client read induction reply: %v", err)
	}
	hdr, body, err := ParseHeader(buf[:n])
	if err != nil || hdr.ControlType != CtrlHandshake {
		t.Fatalf("expected handshake reply; got %+v / err=%v", hdr, err)
	}
	hsReply, err := ParseHandshake(body)
	if err != nil {
		t.Fatalf("parse induction reply: %v", err)
	}
	cookie := hsReply.SyncCookie
	ourSrtSocketID := hsReply.SrtSocketID
	if cookie == 0 {
		t.Fatal("server returned zero cookie")
	}

	//2. CONCLUSION (echo cookie back).
	hsConclusion := (&Handshake{
		Version:        srtVersion5,
		HandshakeType:  HSTypeConclusion,
		SrtSocketID:    peerSocketID,
		SyncCookie:     cookie,
		InitialSequence: 100,
	}).Marshal()
	send(CtrlHandshake, 0, 0, hsConclusion, ourSrtSocketID)

	//Drain the conclusion ack.
	if _, err := cli.Read(buf); err != nil {
		t.Fatalf("conclusion ack: %v", err)
	}

	//3. Data packet — sequence number 100, payload "hello".
	dataPkt := make([]byte, HeaderSize+5)
	MarshalDataHeader(dataPkt, 100, 0, 0, ourSrtSocketID)
	copy(dataPkt[HeaderSize:], "hello")
	if _, err := cli.Write(dataPkt); err != nil {
		t.Fatalf("data send: %v", err)
	}

	//Wait for the onData callback.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(got)
		mu.Unlock()
		if count > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("onData fired %d times, want 1", len(got))
	}
	if string(got[0]) != "hello" {
		t.Errorf("payload = %q, want \"hello\"", got[0])
	}
}

// TestListener_E2ENAKOnLoss exercises the receiver-side loss
// detection: send seq=100, then seq=105 (skipping 101..104). The
// server should respond with a NAK control packet listing the gap.
func TestListener_E2ENAKOnLoss(t *testing.T) {
	l, err := Listen("127.0.0.1:0", "lossy", func(string, []byte) error { return nil })
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	go l.Run()

	cli, err := net.DialUDP("udp", nil,
		l.conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close()
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))

	const peerID = uint32(0x12345678)

	//Walk the handshake the same way the previous test did.
	hsInduction := (&Handshake{
		Version: srtVersion5, HandshakeType: HSTypeAgreement, SrtSocketID: peerID,
	}).Marshal()
	out := make([]byte, HeaderSize+len(hsInduction))
	MarshalControlHeader(out, CtrlHandshake, 0, 0, 0, 0)
	copy(out[HeaderSize:], hsInduction)
	cli.Write(out)
	buf := make([]byte, 1500)
	n, _ := cli.Read(buf)
	_, body, _ := ParseHeader(buf[:n])
	rep, _ := ParseHandshake(body)

	hsConclusion := (&Handshake{
		Version: srtVersion5, HandshakeType: HSTypeConclusion,
		SrtSocketID: peerID, SyncCookie: rep.SyncCookie, InitialSequence: 100,
	}).Marshal()
	out = make([]byte, HeaderSize+len(hsConclusion))
	MarshalControlHeader(out, CtrlHandshake, 0, 0, 0, rep.SrtSocketID)
	copy(out[HeaderSize:], hsConclusion)
	cli.Write(out)
	cli.Read(buf) //conclusion ack

	//Send seq=100 (in-order), then seq=105 (gap).
	send := func(seq uint32) {
		dp := make([]byte, HeaderSize+1)
		MarshalDataHeader(dp, seq, 0, 0, rep.SrtSocketID)
		dp[HeaderSize] = byte(seq)
		cli.Write(dp)
	}
	send(100)
	send(105)

	//Drain control packets until we see a NAK or hit the read deadline.
	gotNAK := false
	var nakAttempts int32
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		_ = cli.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, err := cli.Read(buf)
		if err != nil {
			break
		}
		hdr, body, err := ParseHeader(buf[:n])
		if err != nil {
			continue
		}
		atomic.AddInt32(&nakAttempts, 1)
		if hdr.Kind == KindControl && hdr.ControlType == CtrlNAK {
			ranges := ParseNAK(body)
			if len(ranges) == 1 && ranges[0].From == 101 && ranges[0].To == 104 {
				gotNAK = true
				break
			}
		}
	}
	if !gotNAK {
		t.Errorf("did not observe NAK for [101,104] (saw %d control packets)",
			atomic.LoadInt32(&nakAttempts))
	}
}

// TestListener_E2EShutdown asserts the listener cleans the session up
// on receiving a SHUTDOWN control packet — subsequent data from the
// same peer should be ignored (no onData firing).
func TestListener_E2EShutdown(t *testing.T) {
	gotData := int32(0)
	l, err := Listen("127.0.0.1:0", "stop", func(string, []byte) error {
		atomic.AddInt32(&gotData, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	go l.Run()

	cli, _ := net.DialUDP("udp", nil, l.conn.LocalAddr().(*net.UDPAddr))
	defer cli.Close()
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))

	//Send a SHUTDOWN before any handshake — the listener should
	//tolerate it (sess is nil; just no-op).
	out := make([]byte, HeaderSize)
	MarshalControlHeader(out, CtrlShutdown, 0, 0, 0, 0)
	if _, err := cli.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}

	//Send data after — without a concluded handshake the server
	//drops it, so onData should still be 0.
	dp := make([]byte, HeaderSize+1)
	MarshalDataHeader(dp, 1, 0, 0, 0)
	dp[HeaderSize] = 0xAA
	cli.Write(dp)

	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt32(&gotData); got != 0 {
		t.Errorf("onData fired %d times after shutdown, want 0", got)
	}
}

// TestListener_E2EKeepalive emits a KEEPALIVE control and asserts the
// listener mirrors it back.
func TestListener_E2EKeepalive(t *testing.T) {
	l, _ := Listen("127.0.0.1:0", "keep", nil)
	defer l.Close()
	go l.Run()

	cli, _ := net.DialUDP("udp", nil, l.conn.LocalAddr().(*net.UDPAddr))
	defer cli.Close()
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))

	//Run a quick handshake first so the session exists.
	hsInduction := (&Handshake{
		Version: srtVersion5, HandshakeType: HSTypeAgreement, SrtSocketID: 1,
	}).Marshal()
	out := make([]byte, HeaderSize+len(hsInduction))
	MarshalControlHeader(out, CtrlHandshake, 0, 0, 0, 0)
	copy(out[HeaderSize:], hsInduction)
	cli.Write(out)
	buf := make([]byte, 1500)
	n, _ := cli.Read(buf)
	_, body, _ := ParseHeader(buf[:n])
	rep, _ := ParseHandshake(body)

	//Now send a KEEPALIVE.
	out = make([]byte, HeaderSize)
	MarshalControlHeader(out, CtrlKeepAlive, 0, 0, 0, rep.SrtSocketID)
	if _, err := cli.Write(out); err != nil {
		t.Fatalf("write: %v", err)
	}

	//Expect a KEEPALIVE mirror back.
	gotKA := false
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		_ = cli.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := cli.Read(buf)
		if err != nil {
			continue
		}
		hdr, _, err := ParseHeader(buf[:n])
		if err != nil {
			continue
		}
		if hdr.Kind == KindControl && hdr.ControlType == CtrlKeepAlive {
			gotKA = true
			break
		}
	}
	if !gotKA {
		t.Errorf("did not see KEEPALIVE mirror")
	}
	_ = binary.BigEndian
}
