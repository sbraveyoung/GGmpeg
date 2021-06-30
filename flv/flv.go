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
