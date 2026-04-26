package librtsp

import (
	"bytes"
	"strings"
	"testing"
)

func TestPackHEVCNAL_Single(t *testing.T) {
	//HEVC IDR slice header: type 19 (IDR_W_RADL).
	nal := []byte{0x26, 0x01, 0xAA, 0xBB}
	out := PackHEVCNAL(nal, 1400)
	if len(out) != 1 || !bytes.Equal(out[0], nal) {
		t.Errorf("expected single-NAL passthrough, got %v", out)
	}
}

func TestPackHEVCNAL_FU(t *testing.T) {
	mtu := 100
	nal := make([]byte, 250)
	nal[0] = 0x26 //type 19 (IDR_W_RADL) — bits 1..6 = 0b010011
	nal[1] = 0x01
	for i := 2; i < len(nal); i++ {
		nal[i] = byte(i)
	}
	out := PackHEVCNAL(nal, mtu)
	if len(out) < 2 {
		t.Fatalf("expected fragmentation, got %d", len(out))
	}
	//Each FU packet starts with FU NAL header (2 bytes type=49) +
	//FU header (1 byte). Verify the first packet's type field.
	first := out[0]
	if (first[0]>>1)&0x3F != 49 {
		t.Errorf("first packet type = %d, want 49 (FU)", (first[0]>>1)&0x3F)
	}
	if first[2]&0x80 == 0 {
		t.Errorf("first FU header missing S-bit")
	}
	if first[2]&0x40 != 0 {
		t.Errorf("first FU header has E-bit")
	}
	last := out[len(out)-1]
	if last[2]&0x40 == 0 {
		t.Errorf("last FU header missing E-bit")
	}
	//Reassemble bodies; should equal nal[2:].
	var rebuilt []byte
	for _, p := range out {
		rebuilt = append(rebuilt, p[3:]...)
	}
	if !bytes.Equal(rebuilt, nal[2:]) {
		t.Errorf("rebuilt body mismatch")
	}
}

func TestHEVCContainsKeyframe(t *testing.T) {
	//Type 19 (IDR_W_RADL) — keyframe. Type 1 (TRAIL_N) — not.
	idr := []byte{0x26, 0x01}                  //type=19
	trail := []byte{0x02, 0x01}                //type=1
	if !HEVCContainsKeyframe([][]byte{idr}) {
		t.Error("type 19 should be keyframe")
	}
	if HEVCContainsKeyframe([][]byte{trail}) {
		t.Error("type 1 should not be keyframe")
	}
}

func TestPackOpusFrame(t *testing.T) {
	frame := []byte{0xFC, 0xFF, 0xFE}
	got := PackOpusFrame(frame)
	if !bytes.Equal(got, frame) {
		t.Errorf("Opus pack should be passthrough; got %x want %x", got, frame)
	}
}

func TestBuildSDP_HEVC(t *testing.T) {
	body := string(BuildSDP(SDPParams{
		StreamID: "x",
		HasVideo: true,
		IsHEVC:   true,
		VPS:      []byte{0x40, 0x01, 0x0C, 0x01},
		SPS:      []byte{0x42, 0x01, 0x01},
		PPS:      []byte{0x44, 0x01, 0xC0},
	}))
	for _, want := range []string{
		"a=rtpmap:96 H265/90000",
		"sprop-vps=",
		"sprop-sps=",
		"sprop-pps=",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("HEVC SDP missing %q\n%s", want, body)
		}
	}
}

func TestBuildSDP_Opus(t *testing.T) {
	body := string(BuildSDP(SDPParams{
		StreamID:   "x",
		HasVideo:   true, //needed so SDP isn't entirely empty
		SPS:        []byte{0x67, 0x42, 0xC0, 0x1E},
		PPS:        []byte{0x68, 0xCE, 0x06},
		HasAudio:   true,
		IsOpus:     true,
		AudioChans: 2,
	}))
	for _, want := range []string{
		"a=rtpmap:97 opus/48000/2",
		"sprop-stereo=1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Opus SDP missing %q\n%s", want, body)
		}
	}
}
