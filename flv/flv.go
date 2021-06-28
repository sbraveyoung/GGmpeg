package flv

import (
	"encoding/binary"
	"errors"
	"fmt"

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

type TagBase struct {
	TagType   uint8
	DataSize  uint32 //uint24
	TimeStamp uint32
	StreamID  uint32 //uint24, always 0
	Data      []byte
}

type Tag interface{}

func ParseTag(er easyio.EasyReader) (tag *Tag, err error) {
	b, err := er.ReadN(11)
	if err != nil {
		return
	}

	fmt.Printf("b in tag:%x\n", b)
	tag = &TagBase{
		TagType:   b[0],
		DataSize:  uint32(0x00)<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]),
		TimeStamp: uint32(b[7])<<24 | uint32(b[4])<<16 | uint32(b[5])<<8 | uint32(b[6]),
		StreamID:  uint32(0x00)<<24 | uint32(b[8])<<16 | uint32(b[9])<<8 | uint32(b[10]),
	}
	fmt.Printf("tag:%+v\n", *tag)
	tag.Data, err = er.ReadN(tag.DataSize)
	return tag, err
}

func (tag *Tag) Marshal() (b []byte) {
	b = make([]byte, 0, 11+len(tag.Data))

	b = append(b, tag.TagType)
	b = append(b, uint8(tag.DataSize>>16)&0xff, uint8(tag.DataSize>>8)&0xff, uint8(tag.DataSize&0xff))
	b = append(b, uint8(tag.TimeStamp>>16)&0xff, uint8(tag.TimeStamp>>8)&0xff, uint8(tag.TimeStamp&0xff))
	b = append(b, uint8(tag.TimeStamp>>24)&0xff)
	b = append(b, uint8(tag.StreamID>>16)&0xff, uint8(tag.StreamID>>8)&0xff, uint8(tag.StreamID&0xff))
	b = append(b, tag.Data...)
	return b
}
