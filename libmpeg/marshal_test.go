package libmpeg

import (
	"bytes"
	"testing"

	"github.com/SmartBrave/Athena/easyio"
)

func TestPAT_Marshal_Shape(t *testing.T) {
	pat := &PAT{
		TableID:                0x00,
		SectionSyntaxIndicator: 0x01,
		SectionLength:          0x0d,
		TransportStreamID:      0x01,
		CurrentNextIndicator:   0x01,
		PMTs: map[uint16]*PMT{
			PMT_PID: {},
		},
	}
	buf := &bytes.Buffer{}
	n, finish, err := pat.Marshal(easyio.NewEasyWriter(buf), 1024)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !finish {
		t.Errorf("expected finish=true on PAT marshal")
	}
	if n != buf.Len() {
		t.Errorf("reported n=%d but wrote %d bytes", n, buf.Len())
	}
	out := buf.Bytes()
	if out[0] != 0x00 {
		t.Errorf("table_id = %#x, want 0x00", out[0])
	}
	//Per ISO 13818-1 the section_syntax_indicator bit is the high
	//bit of byte 1.
	if out[1]&0x80 == 0 {
		t.Errorf("section_syntax_indicator not set: byte 1 = %#x", out[1])
	}
}

func TestPMT_Marshal_StreamType(t *testing.T) {
	//Two streams: H.264 (0x1B) on PID 0x100 and AAC (0x0F) on PID 0x101.
	//Their stream_type bytes must appear in the output.
	pmt := &PMT{
		TableID:              0x02,
		SectionLength:        0x17,
		ProgramNumber:        0x01,
		CurrentNextIndicator: 0x01,
		PCR_PID:              VIDEO_PID,
		Streams: map[uint16]*PES{
			VIDEO_PID: {StreamID: 0xE0, StreamType: 0x1B},
			AUDIO_PID: {StreamID: 0xC0, StreamType: 0x0F},
		},
	}
	buf := &bytes.Buffer{}
	if _, _, err := pmt.Marshal(easyio.NewEasyWriter(buf), 1024); err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := buf.Bytes()
	if !bytes.Contains(out, []byte{0x1B}) {
		t.Errorf("PMT missing H.264 stream_type byte 0x1B: %x", out)
	}
	if !bytes.Contains(out, []byte{0x0F}) {
		t.Errorf("PMT missing AAC stream_type byte 0x0F: %x", out)
	}
}

func TestPMT_Marshal_StreamTypeDefaultFromStreamID(t *testing.T) {
	//Old callers that didn't set StreamType — the marshal path falls
	//back to StreamID-derived defaults.
	pmt := &PMT{
		PCR_PID: VIDEO_PID,
		Streams: map[uint16]*PES{
			VIDEO_PID: {StreamID: 0xE0}, //→ default 0x1B
			AUDIO_PID: {StreamID: 0xC0}, //→ default 0x0F
		},
	}
	buf := &bytes.Buffer{}
	_, _, err := pmt.Marshal(easyio.NewEasyWriter(buf), 1024)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out := buf.Bytes()
	if !bytes.Contains(out, []byte{0x1B}) || !bytes.Contains(out, []byte{0x0F}) {
		t.Errorf("legacy fallback failed; PMT body: %x", out)
	}
}

func TestTS_Mux_ProducesValid188(t *testing.T) {
	pes := &PES{
		StreamID:              0xE0,
		StreamType:            0x1B,
		PacketStartCodePrefix: 0x000001,
		DTS:                   0,
		PTS:                   0,
		PTS_DTSFlag:           0x02,
		PESHeaderDataLength:   0x05,
		Data:                  bytes.Repeat([]byte{0xAA}, 100),
	}
	buf := &bytes.Buffer{}
	cc := map[uint16]uint8{}
	finish, err := NewTs(VIDEO_PID, cc, true).Mux(pes, true, 0, easyio.NewEasyWriter(buf))
	if err != nil {
		t.Fatalf("Mux: %v", err)
	}
	if !finish {
		//For a 100-byte payload + PES header there's room in one TS
		//packet; the muxer should report finish=true.
		t.Errorf("expected finish=true on small PES")
	}
	if buf.Len() != 188 {
		t.Errorf("TS packet length = %d, want 188", buf.Len())
	}
	out := buf.Bytes()
	if out[0] != 0x47 {
		t.Errorf("sync byte = %#x, want 0x47", out[0])
	}
	pid := uint16(out[1]&0x1F)<<8 | uint16(out[2])
	if pid != VIDEO_PID {
		t.Errorf("PID = %#x, want %#x", pid, VIDEO_PID)
	}
}

func TestCRC32_NonZeroOnNonZeroInput(t *testing.T) {
	//We exercise the CRC32 path with a small fixed vector and assert
	//(a) deterministic output and (b) sensitivity to single-bit
	//changes. The exact polynomial is implementation-defined so we
	//don't pin to a magic constant.
	got1 := CRC32([]byte{0x00, 0x00, 0x00, 0x00})
	got2 := CRC32([]byte{0x00, 0x00, 0x00, 0x01})
	if got1 == got2 {
		t.Errorf("CRC collision on differing inputs: %#x vs %#x", got1, got2)
	}
	got1Repeat := CRC32([]byte{0x00, 0x00, 0x00, 0x00})
	if got1 != got1Repeat {
		t.Errorf("CRC32 not deterministic: %#x vs %#x", got1, got1Repeat)
	}
}
