package libavc

import (
	"bytes"
	"testing"
)

func TestParser_ParseSpecificInfo(t *testing.T) {
	//Hand-built AVCDecoderConfigurationRecord: SPS+PPS only, with
	//tiny synthetic NAL bodies for parsing-shape verification.
	sps := []byte{0x67, 0x42, 0xC0, 0x1E, 0x91, 0x40}
	pps := []byte{0x68, 0xCE, 0x06, 0xE2}
	dcr := []byte{
		0x01,        //configurationVersion
		0x42,        //AVCProfileIndication
		0xC0,        //profile_compatibility
		0x1E,        //AVCLevelIndication
		0xFF,        //6 bits reserved | lengthSizeMinusOne=3
		0xE1,        //3 bits reserved | numSPS=1
		byte(len(sps) >> 8), byte(len(sps)),
	}
	dcr = append(dcr, sps...)
	dcr = append(dcr, 0x01) //numPPS
	dcr = append(dcr, byte(len(pps)>>8), byte(len(pps)))
	dcr = append(dcr, pps...)

	p := &Parser{Pps: bytes.NewBuffer(make([]byte, MaxSpsPpsLen))}
	if err := p.ParseSpecificInfo(dcr); err != nil {
		t.Fatalf("ParseSpecificInfo: %v", err)
	}
	//SpecificInfo holds startcode + SPS + startcode + PPS.
	if !bytes.Contains(p.SpecificInfo, sps) {
		t.Errorf("SpecificInfo missing SPS")
	}
	if !bytes.Contains(p.SpecificInfo, pps) {
		t.Errorf("SpecificInfo missing PPS")
	}
	//Each set should be preceded by an AnnexB start code 0x00000001.
	startCount := bytes.Count(p.SpecificInfo, []byte{0x00, 0x00, 0x00, 0x01})
	if startCount < 2 {
		t.Errorf("expected ≥2 start codes, got %d", startCount)
	}
}

func TestParser_ParseSpecificInfo_TooShort(t *testing.T) {
	p := &Parser{Pps: bytes.NewBuffer(nil)}
	if err := p.ParseSpecificInfo([]byte{0x01, 0x42}); err == nil {
		t.Error("expected error on truncated DCR")
	}
}

func TestParser_IsNaluHeader(t *testing.T) {
	p := &Parser{}
	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"4-byte start code", []byte{0x00, 0x00, 0x00, 0x01, 0x65}, true},
		{"3-byte start code is not NALU", []byte{0x00, 0x00, 0x01, 0x65, 0x00}, false},
		{"AVCC-style length prefix", []byte{0x00, 0x00, 0x00, 0x42}, false},
		{"too short", []byte{0x00, 0x00}, false},
	}
	for _, c := range cases {
		if got := p.IsNaluHeader(c.in); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParser_GetAnnexbH264_AVCCToAnnexB(t *testing.T) {
	//Pre-load SPS/PPS into the parser so the IDR fast-path emits them.
	sps := []byte{0x67, 0x42, 0xC0, 0x1E}
	pps := []byte{0x68, 0xCE, 0x06}
	dcr := []byte{
		0x01, 0x42, 0xC0, 0x1E,
		0xFF, 0xE1,
		byte(len(sps) >> 8), byte(len(sps)),
	}
	dcr = append(dcr, sps...)
	dcr = append(dcr, 0x01)
	dcr = append(dcr, byte(len(pps)>>8), byte(len(pps)))
	dcr = append(dcr, pps...)
	p := &Parser{Pps: bytes.NewBuffer(make([]byte, MaxSpsPpsLen))}
	if err := p.ParseSpecificInfo(dcr); err != nil {
		t.Fatalf("ParseSpecificInfo: %v", err)
	}

	//AVCC blob: one IDR NAL (type 5).
	idr := []byte{0x65, 0x88, 0x82, 0x0B, 0x44} //fake bytes, type 5 in low 5 bits of 0x65
	avccLen := []byte{0, 0, 0, byte(len(idr))}
	src := append(avccLen, idr...)

	out := &bytes.Buffer{}
	if err := p.GetAnnexbH264(src, out); err != nil {
		t.Fatalf("GetAnnexbH264: %v", err)
	}
	got := out.Bytes()
	//Must include AUD (0x00 0x00 0x00 0x01 0x09), SPS+PPS (because IDR),
	//and the IDR slice itself.
	if !bytes.Contains(got, []byte{0x00, 0x00, 0x00, 0x01, 0x09}) {
		t.Errorf("output missing AUD")
	}
	if !bytes.Contains(got, sps) {
		t.Errorf("output missing SPS")
	}
	if !bytes.Contains(got, pps) {
		t.Errorf("output missing PPS")
	}
	if !bytes.Contains(got, idr) {
		t.Errorf("output missing IDR slice")
	}
}
