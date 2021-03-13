package rtmp

import (
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

type MessageHeaderType uint8

const (
	FMT0 MessageHeaderType = iota
	FMT1
	FMT2
	FMT3
)

type ChunkBasicHeader struct {
	Fmt  MessageHeaderType //2bits
	CsID uint32
}

func NewChunkBasicHeader(conn *rtmpConn) (cbhp *ChunkBasicHeader, err error) {
	b, err := conn.readN(1)
	if err != nil {
		return nil, errors.Wrap(err, "read chunk header from conn")
	}
	fmt.Printf("basic header:%x\n", b)

	cbhp = &ChunkBasicHeader{
		Fmt: MessageHeaderType((b[0] & 0xc0) >> 6),
	}

	switch csid := b[0] & 0x3f; csid {
	case 0x0:
		b1, err := conn.readN(1)
		if err != nil {
			return nil, errors.Wrap(err, "read basic header fron conn")
		}
		cbhp.CsID = uint32(b1[0]) + 64
	case 0x1:
		b2, err := conn.readN(2)
		if err != nil {
			return nil, errors.Wrap(err, "read basic header fron conn")
		}
		cbhp.CsID = uint32(b2[0]) + uint32(b2[1])*256 + 64
	case 0x2:
		//XXX
	default:
		cbhp.CsID = uint32(csid)
	}
	fmt.Printf("basic header struct:%+v\n", *cbhp)
	return cbhp, nil
}

type ChunkMessageHeader struct {
	MessageTimeStampDelta    uint32 //3bytes
	MessageLength            uint32 //3bytes
	MessageType              uint8
	MessageStreamID          uint32 //little-endian 4bytes
	MessageExtendedTimeStamp uint32
}

func NewChunkMessageHeader(conn *rtmpConn, messageType MessageHeaderType) (cmhp *ChunkMessageHeader, err error) {
	cmhp = &ChunkMessageHeader{}
	switch messageType {
	case FMT0:
		b11, err := conn.readN(11)
		if err != nil {
			return nil, errors.Wrap(err, "read message header from conn")
		}
		cmhp.MessageTimeStampDelta = uint32(0x00)<<24 | uint32(b11[0])<<16 | uint32(b11[1])<<8 | uint32(b11[2])
		cmhp.MessageLength = uint32(0x00)<<24 | uint32(b11[3])<<16 | uint32(b11[4])<<8 | uint32(b11[5])
		cmhp.MessageType = b11[6]
		cmhp.MessageStreamID = binary.LittleEndian.Uint32(b11[7:])
	case FMT1:
		b7, err := conn.readN(7)
		if err != nil {
			return nil, errors.Wrap(err, "read message header from conn")
		}
		cmhp.MessageTimeStampDelta = uint32(0x00)<<24 | uint32(b7[0])<<16 | uint32(b7[1])<<8 | uint32(b7[2])
		cmhp.MessageLength = uint32(0x00)<<24 | uint32(b7[3])<<16 | uint32(b7[4])<<8 | uint32(b7[5])
		cmhp.MessageType = b7[6]
	case FMT2:
		b3, err := conn.readN(3)
		if err != nil {
			return nil, errors.Wrap(err, "read message header from conn")
		}
		cmhp.MessageTimeStampDelta = uint32(0x00)<<24 | uint32(b3[0])<<16 | uint32(b3[1])<<8 | uint32(b3[2])
	case FMT3:
		//XXX
	default:
		return nil, errors.Errorf("invalid fmt:%d", messageType)
	}
	if cmhp.MessageTimeStampDelta == 0xffffff {
		b4, err := conn.readN(4)
		if err != nil {
			return nil, errors.Wrap(err, "read extended timestamp from conn")
		}
		cmhp.MessageExtendedTimeStamp = binary.BigEndian.Uint32(b4)
	}
	fmt.Printf("message header struct:%+v\n", *cmhp)
	return cmhp, nil
}

type ChunkHeader struct {
	ChunkBasicHeader
	ChunkMessageHeader
}

type Chunk struct {
	ChunkHeader
	Payload []byte
}
