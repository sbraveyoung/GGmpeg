package libflv

import (
	"encoding/binary"
	"testing"
)

// stubTag carries just enough state to drive FLVWrite.
type stubTag struct {
	tb   TagBase
	body []byte
}

func (s *stubTag) GetTagInfo() *TagBase { return &s.tb }
func (s *stubTag) Marshal() []byte      { return s.body }
func (s *stubTag) Data() []byte         { return s.body }

// FLVWrite needs the tag to type-assert to one of the concrete tag
// pointer types (*AudioTag/*VideoTag/*MetaTag). Wrap the body via
// AudioTag for testing.
func makeAudioTag(ts uint32) *AudioTag {
	return &AudioTag{
		TagBase: TagBase{
			TagType:   AUDIO_TAG,
			DataSize:  1,
			TimeStamp: ts,
		},
		SoundFormat: FLV_AUDIO_AAC,
		SoundData:   []byte{},
	}
}

// TestFLVWriteTimestampExtended ensures the 4th byte of the 11-byte
// FLV tag header (TimestampExtended) carries the high 8 bits of the
// 32-bit timestamp rather than being hardcoded to 0.
func TestFLVWriteTimestampExtended(t *testing.T) {
	const bigTS = uint32(0x12345678) //high byte = 0x12
	tag := makeAudioTag(bigTS)
	out := FLVWrite(tag)

	//Tag header: 1 byte type + 3 bytes datasize + 3 bytes ts low + 1
	//byte ts ext + 3 bytes streamid = 11 bytes.
	if len(out) < 11 {
		t.Fatalf("FLVWrite output too short: %d bytes", len(out))
	}
	tsLow := uint32(out[4])<<16 | uint32(out[5])<<8 | uint32(out[6])
	tsExt := uint32(out[7])
	got := tsExt<<24 | tsLow
	if got != bigTS {
		t.Errorf("encoded timestamp = %#x, want %#x (tsLow=%#x tsExt=%#x)",
			got, bigTS, tsLow, tsExt)
	}
}

// TestFLVWritePreviousTagSize asserts the 4-byte trailer equals the
// total tag size (11-byte header + body).
func TestFLVWritePreviousTagSize(t *testing.T) {
	tag := makeAudioTag(0)
	tag.SoundData = []byte{0xaa, 0xbb, 0xcc}
	out := FLVWrite(tag)

	prev := binary.BigEndian.Uint32(out[len(out)-4:])
	wantPrev := uint32(len(out) - 4)
	if prev != wantPrev {
		t.Errorf("previous tag size = %d, want %d", prev, wantPrev)
	}
}
