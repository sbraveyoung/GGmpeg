package libflv

import (
	"encoding/binary"

	"github.com/SmartBrave/Athena/easyio"
)

const (
	AUDIO_TAG       uint8 = 8
	VIDEO_TAG       uint8 = 9
	SCRIPT_DATA_TAG uint8 = 18
)

type FLVHeader struct {
	Version        int8
	TypeFlagsAudio bool
	TypeFlagsVideo bool
}

type FLVBody struct {
	//TODO
}

type FLV struct {
	FLVHeader
	writer easyio.EasyWriter
}

func NewFLV(writer easyio.EasyWriter) *FLV {
	return &FLV{
		FLVHeader: FLVHeader{
			Version:        0x01,
			TypeFlagsAudio: true,
			TypeFlagsVideo: true,
		},
		writer: writer,
	}
}

// FLVWrite serialises a single FLV tag (audio / video / script data).
// Per FLV v10.1 §E.4.1, the tag header is:
//
//	1 byte  TagType
//	3 bytes DataSize       (low-order 24 bits of body length)
//	3 bytes Timestamp      (low-order 24 bits)
//	1 byte  TimestampExt   (high 8 bits — forms the full 32-bit stamp)
//	3 bytes StreamID       (always 0)
//
// followed by the body bytes and a 4-byte PreviousTagSize footer.
func FLVWrite(tag Tag) (b []byte) {
	switch tag.(type) {
	case *AudioTag:
		b = append(b, AUDIO_TAG)
	case *VideoTag:
		b = append(b, VIDEO_TAG)
	case *MetaTag:
		b = append(b, SCRIPT_DATA_TAG)
	default:
	}

	data := tag.Marshal()

	//DataSize: low 24 bits.
	var sb [4]byte
	binary.BigEndian.PutUint32(sb[:], uint32(len(data)))
	b = append(b, sb[1], sb[2], sb[3])

	//Timestamp: low 24 bits in the first 3 bytes, high 8 bits in the
	//TimestampExtended byte. Hardcoding the extended byte to 0 (the
	//prior behaviour) silently wraps timestamps past ~4.66 hours.
	ts := tag.GetTagInfo().TimeStamp
	binary.BigEndian.PutUint32(sb[:], ts)
	b = append(b, sb[1], sb[2], sb[3], sb[0])

	//StreamID: always zero per spec.
	b = append(b, 0x0, 0x0, 0x0)

	//Tag body + PreviousTagSize footer.
	b = append(b, data...)
	prev := make([]byte, 4)
	binary.BigEndian.PutUint32(prev, uint32(len(b)))
	b = append(b, prev...)

	return b
}
