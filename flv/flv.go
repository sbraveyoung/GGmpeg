package flv

import (
	"encoding/binary"
	"errors"

	"github.com/SmartBrave/utils/easyio"
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
}

func ParseFLV(er easyio.EasyReader) (flv *FLV, err error) {
	b, err := er.ReadN(9)
	if err != nil {
		return
	}

	if b[0] != 'F' || b[1] != 'L' || b[2] != 'V' ||
		b[3] != 0x01 ||
		b[4]&0xf8 != 0 || b[4]&0x02 != 0 ||
		binary.BigEndian.Uint32(b[5:9]) != 9 {
		err = errors.New("invalid data format")
		return
	}

	flv = &FLV{
		FLVHeader: FLVHeader{
			Version: 1,
		},
	}

	if b[4]&0x04 != 0 {
		flv.TypeFlagsAudio = true
	}
	if b[4]&0x01 != 0 {
		flv.TypeFlagsVideo = true
	}
	return flv, nil
}

type Tag struct {
	TagType   uint8
	DataSize  uint32 //uint24
	TimeStamp uint32
	StreamID  uint32 //uint24, always 0
	Data      []byte
}

func ParseTag(er easyio.EasyReader) (tag *Tag, err error) {
	b, err := er.ReadN(11)
	if err != nil {
		return
	}

	tag = &Tag{
		TagType:   b[0],
		DataSize:  uint32(0x00)<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]),
		TimeStamp: uint32(b[7])<<24 | uint32(b[4])<<16 | uint32(b[5])<<8 | uint32(b[6]),
		StreamID:  uint32(0x00)<<24 | uint32(b[8])<<16 | uint32(b[9])<<8 | uint32(b[10]),
	}
	tag.Data, err = er.ReadN(tag.DataSize)
	return tag, err
}
