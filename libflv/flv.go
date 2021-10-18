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

//func ParseFLV(er easyio.EasyReader) (flv *FLV, err error) {
//	b, err := er.ReadN(9)
//	if err != nil {
//		return
//	}
//
//	if b[0] != 'F' || b[1] != 'L' || b[2] != 'V' ||
//		b[3] != 0x01 ||
//		b[4]&0xf8 != 0 || b[4]&0x02 != 0 ||
//		binary.BigEndian.Uint32(b[5:9]) != 9 {
//		err = errors.New("invalid data format")
//		return
//	}
//
//	flv = &FLV{
//		FLVHeader: FLVHeader{
//			Version: 1,
//		},
//	}
//
//	if b[4]&0x04 != 0 {
//		flv.TypeFlagsAudio = true
//	}
//	if b[4]&0x01 != 0 {
//		flv.TypeFlagsVideo = true
//	}
//	return flv, nil
//}

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

	//Tag
	data := tag.Marshal()

	//////////////////////////XXX maybe do better
	//DataSize
	sb := make([]byte, 4, 4)
	binary.BigEndian.PutUint32(sb, uint32(len(data)))
	b = append(b /*sb[0],*/, sb[1], sb[2], sb[3])

	//Timestamp and TimestampExtended
	binary.BigEndian.PutUint32(sb, uint32(tag.GetTagInfo().TimeStamp))
	b = append(b /*sb[0],*/, sb[1], sb[2], sb[3], 0x0)
	//////////////////////////

	//StreamID
	b = append(b, 0x0, 0x0, 0x0)

	//tag data
	b = append(b, data...)
	writedSizeByte := make([]byte, 4, 4)
	binary.BigEndian.PutUint32(writedSizeByte, uint32(len(b)))
	b = append(b, writedSizeByte...)

	return b
}
