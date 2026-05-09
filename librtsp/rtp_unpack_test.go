package librtsp

import (
	"bytes"
	"testing"
)

func TestParseRTP_Basic(t *testing.T) {
	pkt := []byte{
		0x80, //V=2 P=0 X=0 CC=0
		0x60, //M=0 PT=96
		0x00, 0x05, //seq=5
		0x00, 0x00, 0x10, 0x00, //ts
		0xDE, 0xAD, 0xBE, 0xEF, //ssrc
		0xAA, 0xBB,
	}
	got, err := ParseRTP(pkt)
	if err != nil {
		t.Fatalf("ParseRTP: %v", err)
	}
	if got.PayloadType != 96 {
		t.Errorf("PT = %d, want 96", got.PayloadType)
	}
	if got.SequenceNum != 5 {
		t.Errorf("seq = %d", got.SequenceNum)
	}
	if got.SSRC != 0xDEADBEEF {
		t.Errorf("SSRC = %x", got.SSRC)
	}
	if !bytes.Equal(got.Payload, []byte{0xAA, 0xBB}) {
		t.Errorf("payload = %x", got.Payload)
	}
	if got.Marker {
		t.Error("marker bit set unexpectedly")
	}
}

func TestParseRTP_TruncatedHeader(t *testing.T) {
	if _, err := ParseRTP([]byte{0x80, 0x60}); err == nil {
		t.Error("expected error on too-short RTP")
	}
}

func TestH264Reassembler_FUARoundtrip(t *testing.T) {
	//Take a 250-byte fake NAL, fragment via PackNAL+Pack, re-feed via
	//ParseRTP+H264Reassembler.Push; assert we get the original NAL
	//back at the M-bit packet.
	nal := make([]byte, 250)
	nal[0] = 0x65 //IDR slice
	for i := 1; i < len(nal); i++ {
		nal[i] = byte(i)
	}
	mtu := 100
	packetsRTP := PackNAL(nal, mtu)
	if len(packetsRTP) < 2 {
		t.Fatalf("expected fragmentation, got %d packets", len(packetsRTP))
	}
	packer := NewRTPPacker(96, 0xCAFEBABE)

	var ra H264Reassembler
	for i, payload := range packetsRTP {
		marker := i == len(packetsRTP)-1
		raw := packer.Pack(marker, 100, payload)
		pkt, err := ParseRTP(raw)
		if err != nil {
			t.Fatalf("ParseRTP: %v", err)
		}
		nals, ts := ra.Push(pkt)
		if i < len(packetsRTP)-1 {
			if nals != nil {
				t.Errorf("premature flush at packet %d", i)
			}
			continue
		}
		if len(nals) != 1 {
			t.Fatalf("got %d nals, want 1", len(nals))
		}
		if !bytes.Equal(nals[0], nal) {
			t.Errorf("reassembled mismatch")
		}
		if ts != 100 {
			t.Errorf("ts = %d, want 100", ts)
		}
	}
}

func TestH264Reassembler_SingleNAL(t *testing.T) {
	nal := []byte{0x65, 0x01, 0x02, 0x03}
	pkt := &RTPPacket{Marker: true, Timestamp: 200, Payload: nal}
	var ra H264Reassembler
	nals, ts := ra.Push(pkt)
	if len(nals) != 1 || !bytes.Equal(nals[0], nal) || ts != 200 {
		t.Errorf("got %v ts=%d", nals, ts)
	}
}

func TestAACAUExtract(t *testing.T) {
	//Two AUs of 3 and 5 bytes.
	frames := [][]byte{
		{0x11, 0x22, 0x33},
		{0xAA, 0xBB, 0xCC, 0xDD, 0xEE},
	}
	//AU-headers-length=32 bits = 2 × 16-bit AU header.
	payload := []byte{0x00, 0x20}
	for _, f := range frames {
		size := len(f) << 3
		payload = append(payload, byte(size>>8), byte(size))
	}
	for _, f := range frames {
		payload = append(payload, f...)
	}
	got := AACAUExtract(payload)
	if len(got) != 2 {
		t.Fatalf("got %d frames, want 2", len(got))
	}
	if !bytes.Equal(got[0], frames[0]) || !bytes.Equal(got[1], frames[1]) {
		t.Errorf("frames mismatch: %v", got)
	}
}

func TestAVCCFromNALs(t *testing.T) {
	a := []byte{0x67, 0x42}
	b := []byte{0x68, 0xCE, 0x06}
	want := []byte{0, 0, 0, 2, 0x67, 0x42, 0, 0, 0, 3, 0x68, 0xCE, 0x06}
	if got := AVCCFromNALs([][]byte{a, b}); !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

func TestContainsKeyframe(t *testing.T) {
	if !ContainsKeyframe([][]byte{{0x65}}) {
		t.Error("type 5 should be keyframe")
	}
	if ContainsKeyframe([][]byte{{0x61}}) {
		t.Error("type 1 should not be keyframe")
	}
}
