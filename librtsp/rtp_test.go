package librtsp

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRTPPacker_Header(t *testing.T) {
	p := NewRTPPacker(96, 0xDEADBEEF)
	p.SetSeq(10)
	pkt := p.Pack(true, 0x12345678, []byte{0xAA, 0xBB})

	if len(pkt) != 12+2 {
		t.Fatalf("packet length = %d, want %d", len(pkt), 14)
	}
	if pkt[0]>>6 != 2 {
		t.Errorf("V = %d, want 2", pkt[0]>>6)
	}
	if pkt[1]&0x80 == 0 {
		t.Error("M-bit not set")
	}
	if pkt[1]&0x7F != 96 {
		t.Errorf("PT = %d, want 96", pkt[1]&0x7F)
	}
	if seq := binary.BigEndian.Uint16(pkt[2:]); seq != 10 {
		t.Errorf("Seq = %d, want 10", seq)
	}
	if ts := binary.BigEndian.Uint32(pkt[4:]); ts != 0x12345678 {
		t.Errorf("TS = %#x, want 0x12345678", ts)
	}
	if ssrc := binary.BigEndian.Uint32(pkt[8:]); ssrc != 0xDEADBEEF {
		t.Errorf("SSRC = %#x", ssrc)
	}
	if !bytes.Equal(pkt[12:], []byte{0xAA, 0xBB}) {
		t.Errorf("payload = %x", pkt[12:])
	}
	if p.Seq() != 11 {
		t.Errorf("Seq() after Pack = %d, want 11", p.Seq())
	}
}

func TestInterleaveFrame(t *testing.T) {
	frame := InterleaveFrame(7, []byte{0xCA, 0xFE})
	want := []byte{'$', 7, 0x00, 0x02, 0xCA, 0xFE}
	if !bytes.Equal(frame, want) {
		t.Errorf("got %x, want %x", frame, want)
	}
}

func TestPackNAL_SingleNAL(t *testing.T) {
	nal := []byte{0x65, 0x88, 0xAA, 0xBB} //type 5 (IDR slice)
	out := PackNAL(nal, 1400)
	if len(out) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(out))
	}
	if !bytes.Equal(out[0], nal) {
		t.Errorf("single-NAL payload = %x, want %x", out[0], nal)
	}
}

func TestPackNAL_FUA(t *testing.T) {
	//Build a fake NAL just over MTU so it fragments into 2 packets.
	mtu := 100
	nal := make([]byte, 250)
	nal[0] = 0x65 //NRI=3, type=5 (IDR)
	for i := 1; i < len(nal); i++ {
		nal[i] = byte(i)
	}
	out := PackNAL(nal, mtu)
	if len(out) < 2 {
		t.Fatalf("expected >=2 packets, got %d", len(out))
	}
	//First fragment: S bit set, E bit not set.
	if out[0][1]&0x80 == 0 {
		t.Errorf("first FU-A missing S-bit: %x", out[0][:2])
	}
	if out[0][1]&0x40 != 0 {
		t.Errorf("first FU-A has E-bit: %x", out[0][:2])
	}
	//Last fragment: E bit set.
	last := out[len(out)-1]
	if last[1]&0x40 == 0 {
		t.Errorf("last FU-A missing E-bit: %x", last[:2])
	}
	//FU indicator: NRI from original NAL, type=28 (FU-A).
	wantIndicator := (byte(0x65)&0x60 | 28)
	if out[0][0] != wantIndicator {
		t.Errorf("FU indicator = %x, want %x", out[0][0], wantIndicator)
	}
	//Reassembled payload (everything after the 2-byte FU prefix in
	//each packet) must equal the original NAL body.
	var rebuilt []byte
	for _, p := range out {
		rebuilt = append(rebuilt, p[2:]...)
	}
	if !bytes.Equal(rebuilt, nal[1:]) {
		t.Errorf("rebuilt body doesn't match")
	}
}

func TestSplitAVCC(t *testing.T) {
	a := []byte{0x67, 0x42}
	b := []byte{0x68, 0xCE, 0x06}
	buf := []byte{0, 0, 0, 2}
	buf = append(buf, a...)
	buf = append(buf, 0, 0, 0, 3)
	buf = append(buf, b...)
	nals := SplitAVCC(buf)
	if len(nals) != 2 {
		t.Fatalf("got %d nals, want 2", len(nals))
	}
	if !bytes.Equal(nals[0], a) {
		t.Errorf("nal[0] = %x, want %x", nals[0], a)
	}
	if !bytes.Equal(nals[1], b) {
		t.Errorf("nal[1] = %x, want %x", nals[1], b)
	}
}

func TestPackAACFrame(t *testing.T) {
	frame := []byte{0x21, 0x10, 0x05, 0x00, 0x80, 0x01}
	pkt := PackAACFrame(frame)
	if len(pkt) != 4+len(frame) {
		t.Fatalf("packet length = %d, want %d", len(pkt), 4+len(frame))
	}
	if hdrLen := binary.BigEndian.Uint16(pkt[:2]); hdrLen != 16 {
		t.Errorf("AU-headers-length = %d, want 16", hdrLen)
	}
	auHeader := binary.BigEndian.Uint16(pkt[2:4])
	if size := auHeader >> 3; int(size) != len(frame) {
		t.Errorf("AU-size = %d, want %d", size, len(frame))
	}
	if !bytes.Equal(pkt[4:], frame) {
		t.Errorf("payload mismatch")
	}
}
