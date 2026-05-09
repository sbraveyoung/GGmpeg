package libaac

import (
	"testing"
)

func TestAACHeader_Parse(t *testing.T) {
	//AudioSpecificConfig: AAC-LC (object type 2), 44.1 kHz (index 4),
	//stereo (channel 2). Encoding (per ISO 14496-3 §1.6.2.1):
	//   5 bits objType  = 00010
	//   4 bits sampIdx  = 0100
	//   4 bits channels = 0010
	//   3 bits padding
	//Layout: 00010 0100 0010 000 → 0x12 0x10
	asc := []byte{0x12, 0x10}
	h := &AACHeader{}
	if err := h.Parse(asc); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if h.ObjectType != 2 {
		t.Errorf("ObjectType = %d, want 2 (AAC-LC)", h.ObjectType)
	}
	if h.SampleRate != 4 {
		t.Errorf("SampleRate index = %d, want 4 (44.1 kHz)", h.SampleRate)
	}
	if h.Channel != 2 {
		t.Errorf("Channel = %d, want 2", h.Channel)
	}
	if AACRates[h.SampleRate] != 44100 {
		t.Errorf("AACRates[%d] = %d, want 44100", h.SampleRate, AACRates[h.SampleRate])
	}
}

func TestAACHeader_ParseTooShort(t *testing.T) {
	if err := (&AACHeader{}).Parse([]byte{0x12}); err == nil {
		t.Error("expected error on truncated config")
	}
}

func TestAACHeader_AdtsHeaderShape(t *testing.T) {
	h := &AACHeader{ObjectType: 2, SampleRate: 4, Channel: 2}
	frame := make([]byte, 100)
	hdr := h.Adts(frame)
	if len(hdr) != 7 {
		t.Fatalf("ADTS header length = %d, want 7", len(hdr))
	}
	//Sync word: 12 bits all 1 — bytes 0 and high nibble of byte 1.
	if hdr[0] != 0xFF {
		t.Errorf("byte 0 = %#x, want 0xFF (sync word LSB)", hdr[0])
	}
	if hdr[1]&0xF0 != 0xF0 {
		t.Errorf("byte 1 high nibble = %#x, want 0xF0 (sync word MSB)", hdr[1]&0xF0)
	}
	//profile field bits 6..7 of byte 2: object_type-1 = 1 = MPEG-4 AAC LC.
	if (hdr[2]>>6)&0x03 != 1 {
		t.Errorf("profile = %d, want 1", (hdr[2]>>6)&0x03)
	}
	//Sampling-frequency index bits 2..5 of byte 2.
	if (hdr[2]>>2)&0x0F != 4 {
		t.Errorf("sample rate index = %d, want 4", (hdr[2]>>2)&0x0F)
	}
}

func TestAACHeader_AdtsFrameLengthEncoding(t *testing.T) {
	//ADTS frame_length is 13 bits and equals raw frame size + 7 (the
	//7-byte ADTS header). Spread across bytes 3 (low 2 bits), 4
	//(8 bits), 5 (high 3 bits).
	h := &AACHeader{ObjectType: 2, SampleRate: 4, Channel: 2}
	for _, sz := range []int{16, 1024, 4093} { //must fit in 13 bits − 7
		hdr := h.Adts(make([]byte, sz))
		got := (uint16(hdr[3]&0x03) << 11) |
			(uint16(hdr[4]) << 3) |
			(uint16(hdr[5]>>5) & 0x07)
		want := uint16(sz + 7)
		if got != want {
			t.Errorf("size %d: encoded frame_length = %d, want %d", sz, got, want)
		}
	}
}
