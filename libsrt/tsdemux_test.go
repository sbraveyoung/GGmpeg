package libsrt

import (
	"testing"
)

// buildTSPacket builds a 188-byte MPEG-TS packet with the given PID,
// PUSI flag, continuity counter, and payload. Adaptation field is set
// only when needed to pad to 188 bytes.
func buildTSPacket(pid uint16, pusi bool, cc byte, payload []byte) []byte {
	out := make([]byte, TSPacketSize)
	out[0] = 0x47
	hi := byte(pid >> 8)
	if pusi {
		hi |= 0x40
	}
	out[1] = hi
	out[2] = byte(pid)
	//Adaptation+payload control = 0x10 (payload only) when no padding
	//is needed. With padding we'd use 0x30 + adaptation field.
	pad := TSPacketSize - 4 - len(payload)
	if pad < 0 {
		pad = 0
		payload = payload[:TSPacketSize-4]
	}
	if pad == 0 {
		out[3] = 0x10 | (cc & 0x0F)
		copy(out[4:], payload)
		return out
	}
	out[3] = 0x30 | (cc & 0x0F)
	out[4] = byte(pad - 1) //adaptation_field_length
	if pad > 1 {
		out[5] = 0x00
		for i := 6; i < 4+pad; i++ {
			out[i] = 0xFF
		}
	}
	copy(out[4+pad:], payload)
	return out
}

func TestDemuxer_PESReassembly(t *testing.T) {
	d := NewDemuxer()
	d.videoPID = 0x100 //skip PAT/PMT for the test
	var got []AccessUnit
	d.OnVideo = func(au AccessUnit) {
		got = append(got, au)
	}

	//Synthesize one PES with PUSI (header carrying PTS) + body NAL.
	//PES: [00 00 01 e0 LL LL 80 80 05 PTS×5][NAL bytes]
	pesHdr := []byte{0x00, 0x00, 0x01, 0xE0, 0x00, 0x00,
		0x80, 0x80, 0x05,
		0x21, 0x00, 0x01, 0x00, 0x01, //PTS=0
	}
	nal := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAB, 0xCD, 0xEF}
	pes := append(pesHdr, nal...)

	//First TS packet carries PUSI + as much of the PES as fits.
	first := buildTSPacket(d.videoPID, true, 0, pes)
	d.Feed(first)

	//Second packet (with PUSI=true) flushes the previous PES.
	flushPES := []byte{0x00, 0x00, 0x01, 0xE0, 0x00, 0x00,
		0x80, 0x80, 0x05,
		0x21, 0x00, 0x07, 0xFF, 0xFF}
	flushNAL := []byte{0x00, 0x00, 0x00, 0x01, 0x61, 0x99}
	flushPES = append(flushPES, flushNAL...)
	second := buildTSPacket(d.videoPID, true, 1, flushPES)
	d.Feed(second)

	if len(got) != 1 {
		t.Fatalf("got %d access units, want 1", len(got))
	}
	if !got[0].Key {
		t.Errorf("first AU should be a keyframe (NAL type 5)")
	}
}

func TestContainsAnnexBKeyframe(t *testing.T) {
	keyAU := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAB}
	if !containsAnnexBKeyframe(keyAU) {
		t.Error("type 5 should be detected as keyframe")
	}
	nonKey := []byte{0x00, 0x00, 0x00, 0x01, 0x61, 0xAB}
	if containsAnnexBKeyframe(nonKey) {
		t.Error("type 1 should not be flagged as keyframe")
	}
}

func TestParsePESHeader_PTSOnly(t *testing.T) {
	pes := []byte{0x00, 0x00, 0x01, 0xE0, 0x00, 0x00,
		0x80, 0x80, 0x05,
		0x21, 0x00, 0x01, 0x00, 0x01,
		0xAA, 0xBB,
	}
	pts, dts, body, ok := parsePESHeader(pes)
	if !ok {
		t.Fatal("parsePESHeader failed")
	}
	if len(body) != 2 {
		t.Errorf("body = %x, want 2 bytes", body)
	}
	if pts != dts {
		t.Errorf("PTS != DTS but only PTS flag was set: pts=%d dts=%d", pts, dts)
	}
	_ = pts
}

func TestParsePESHeader_PTSDTS(t *testing.T) {
	pes := []byte{0x00, 0x00, 0x01, 0xE0, 0x00, 0x00,
		0x80, 0xC0, 0x0A,
		0x31, 0x00, 0x01, 0x00, 0x01, //PTS
		0x11, 0x00, 0x01, 0x00, 0x01, //DTS
		0xAA, 0xBB,
	}
	_, _, body, ok := parsePESHeader(pes)
	if !ok {
		t.Fatal("parsePESHeader failed")
	}
	if len(body) != 2 {
		t.Errorf("body = %x, want 2 bytes", body)
	}
}
