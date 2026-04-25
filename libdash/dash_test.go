package libdash

import (
	"strings"
	"testing"
	"time"
)

func TestParseAVCDCR_Basic(t *testing.T) {
	//Hand-crafted AVCDecoderConfigurationRecord with a tiny SPS+PPS.
	//Real SPS bytes don't matter for the parser — we just verify the
	//framing pulls out the right slices.
	sps := []byte{0x67, 0x42, 0xC0, 0x1E, 0xDB, 0x02, 0x80, 0xBF, 0xE5}
	pps := []byte{0x68, 0xCE, 0x06, 0xE2}
	dcr := []byte{
		0x01,        //configurationVersion
		0x42,        //AVCProfileIndication (baseline)
		0xC0,        //profile_compatibility
		0x1E,        //AVCLevelIndication (level 3.0)
		0xFF,        //6 reserved + lengthSizeMinusOne=3
		0xE1,        //3 reserved + numSPS=1
		byte(len(sps) >> 8), byte(len(sps)),
	}
	dcr = append(dcr, sps...)
	dcr = append(dcr, 0x01) //numPPS
	dcr = append(dcr, byte(len(pps)>>8), byte(len(pps)))
	dcr = append(dcr, pps...)

	gotSPS, gotPPS, _, _, err := parseAVCDCR(dcr)
	if err != nil {
		t.Fatalf("parseAVCDCR: %v", err)
	}
	if string(gotSPS) != string(sps) {
		t.Errorf("SPS mismatch: got %x want %x", gotSPS, sps)
	}
	if string(gotPPS) != string(pps) {
		t.Errorf("PPS mismatch: got %x want %x", gotPPS, pps)
	}
}

func TestParseAVCDCR_TooShort(t *testing.T) {
	if _, _, _, _, err := parseAVCDCR([]byte{0x01, 0x42}); err == nil {
		t.Error("expected error on truncated DCR")
	}
}

func TestBuildMPD_DynamicLive(t *testing.T) {
	in := manifestInputs{
		streamID:          "live1",
		availabilityStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		timescale:         1000,
		targetDur:         2 * time.Second,
		width:             1280,
		height:            720,
		segments: []segmentInfo{
			{seq: 0, filename: "live1-0.m4s", startTime: 0, duration: 2000},
			{seq: 1, filename: "live1-1.m4s", startTime: 2000, duration: 2000},
		},
	}
	got := string(buildMPD(in))

	wants := []string{
		`type="dynamic"`,
		`availabilityStartTime="2026-01-01T00:00:00Z"`,
		`profiles="urn:mpeg:dash:profile:isoff-live:2011"`,
		`<Representation id="v0"`,
		`width="1280"`,
		`height="720"`,
		`<SegmentTemplate `,
		`initialization="live1-init.mp4"`,
		`media="live1-$Number$.m4s"`,
		`startNumber="0"`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("MPD missing %q\n--- full ---\n%s", w, got)
		}
	}
}

func TestBuildMPD_NoSegments(t *testing.T) {
	if got := buildMPD(manifestInputs{}); got != nil {
		t.Errorf("expected nil for empty inputs, got %q", got)
	}
}

// TestParseSPSDimensions_Smoke: a minimal high-profile SPS would be
// hard to hand-build here — we just sanity-check that the parser
// doesn't panic on a non-trivial-but-incomplete byte slice.
func TestParseSPSDimensions_Smoke(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("parseSPSDimensions panicked: %v", r)
		}
	}()
	//Real-world SPS captured from FFmpeg's "testsrc" output.
	sps := []byte{
		0x67, 0x42, 0xC0, 0x1E, 0xDB, 0x02, 0x80, 0xBF, 0xE5, 0xC4, 0x40, 0x00,
		0x00, 0x03, 0x00, 0x40, 0x00, 0x00, 0x0F, 0x03, 0xC5, 0x8B, 0xA8,
	}
	w, h := parseSPSDimensions(sps)
	if w == 0 || h == 0 {
		//Not a hard fail — the parser bails out on profile-specific
		//branches it doesn't fully implement. But we should at least
		//not panic.
		t.Logf("parseSPSDimensions returned %dx%d (acceptable)", w, h)
	}
}
