package rtmp

import (
	"encoding/binary"
	"fmt"

	"github.com/SmartBrave/utils/easyerrors"
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
	Fmt  MessageHeaderType //2 bits
	CsID uint32            //6, 14 or 22 bits
}

func parseChunkBasicHeader(rtmp *RTMP) (cbhp *ChunkBasicHeader, err error) {
	b, err := rtmp.conn.ReadN(1)
	if err != nil {
		return nil, err
	}
	fmt.Printf("basic header:%x\n", b)

	cbhp = &ChunkBasicHeader{
		Fmt: MessageHeaderType((b[0] & 0xc0) >> 6),
	}

	switch csid := b[0] & 0x3f; csid {
	case 0x0:
		b1, err := rtmp.conn.ReadN(1)
		if err != nil {
			return nil, err
		}
		cbhp.CsID = uint32(b1[0]) + 64
	case 0x1:
		b2, err := rtmp.conn.ReadN(2)
		if err != nil {
			return nil, err
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
	MessageTimeStamp uint32 //3bytes or 4bytes(extended timestamp)
	MessageLength    int    //3bytes
	MessageType      MessageType
	MessageStreamID  uint32 //little-endian 4bytes
}

func parseChunkMessageHeader(rtmp *RTMP, messageType MessageHeaderType) (cmhp *ChunkMessageHeader, err error) {
	if messageType != FMT0 && rtmp.lastChunk == nil {
		return nil, errors.Errorf("invalid fmt_a:%d", messageType)
	}

	cmhp = &ChunkMessageHeader{}
	switch messageType {
	case FMT0:
		b11, err := rtmp.conn.ReadN(11)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = uint32(0x00)<<24 | uint32(b11[0])<<16 | uint32(b11[1])<<8 | uint32(b11[2])
		cmhp.MessageLength = int(0x00)<<24 | int(b11[3])<<16 | int(b11[4])<<8 | int(b11[5])
		cmhp.MessageType = MessageType(b11[6])
		cmhp.MessageStreamID = binary.LittleEndian.Uint32(b11[7:])
	case FMT1:
		b7, err := rtmp.conn.ReadN(7)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = uint32(0x00)<<24 | uint32(b7[0])<<16 | uint32(b7[1])<<8 | uint32(b7[2])
		cmhp.MessageLength = int(0x00)<<24 | int(b7[3])<<16 | int(b7[4])<<8 | int(b7[5])
		cmhp.MessageType = MessageType(b7[6])
		cmhp.MessageStreamID = rtmp.lastChunk.MessageStreamID
	case FMT2:
		b3, err := rtmp.conn.ReadN(3)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = uint32(0x00)<<24 | uint32(b3[0])<<16 | uint32(b3[1])<<8 | uint32(b3[2])
		cmhp.MessageLength = rtmp.lastChunk.MessageLength
		cmhp.MessageType = rtmp.lastChunk.MessageType
		cmhp.MessageStreamID = rtmp.lastChunk.MessageStreamID
	case FMT3:
		cmhp.MessageTimeStamp = rtmp.lastChunk.MessageTimeStamp
		cmhp.MessageLength = rtmp.lastChunk.MessageLength
		cmhp.MessageType = rtmp.lastChunk.MessageType
		cmhp.MessageStreamID = rtmp.lastChunk.MessageStreamID
	default:
		return nil, errors.Errorf("invalid fmt_b:%d", messageType)
	}
	if cmhp.MessageTimeStamp == 0xffffff {
		b4, err := rtmp.conn.ReadN(4)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = binary.BigEndian.Uint32(b4)
	}
	fmt.Printf("message header struct:%+v\n", *cmhp)
	return cmhp, nil
}

type Chunk struct {
	ChunkBasicHeader
	ChunkMessageHeader
	Payload []byte
}

func ParseChunk(rtmp *RTMP, message Message) (cp *Chunk, err error) {
	basicHeader, err := parseChunkBasicHeader(rtmp)
	if err != nil {
		return nil, err
	}

	messageHeader, err := parseChunkMessageHeader(rtmp, basicHeader.Fmt)
	if err != nil {
		return nil, err
	}

	chunkSize := messageHeader.MessageLength
	if chunkSize > rtmp.maxChunkSize {
		chunkSize = rtmp.maxChunkSize
	}
	if message != nil {
		if remain := message.Done(); chunkSize > remain {
			chunkSize = remain
		}
	}
	b := make([]byte, chunkSize)
	err = rtmp.conn.ReadFull(b)
	if err != nil {
		return nil, err
	}
	cp = &Chunk{
		ChunkBasicHeader:   *basicHeader,
		ChunkMessageHeader: *messageHeader,
		Payload:            b,
	}
	rtmp.lastChunk = cp
	return cp, nil
}

//NOTE: ensure len(payload) <= maxChunkSize
func NewChunk(messageType MessageType, fmt MessageHeaderType, payload []byte) (chunk *Chunk) {
	return &Chunk{
		ChunkBasicHeader: ChunkBasicHeader{
			Fmt:  fmt,
			CsID: 2,
		},
		ChunkMessageHeader: ChunkMessageHeader{
			MessageTimeStamp: 0,
			MessageLength:    len(payload),
			MessageType:      messageType,
			MessageStreamID:  0,
		},
		Payload: payload,
	}
}

func (chunk *Chunk) Send(rtmp *RTMP) (err error) {
	b := []byte{}
	b = append(b, byte(uint8(chunk.Fmt<<6)|uint8(chunk.CsID&0x3f)))
	switch chunk.Fmt {
	case FMT0:
		b = append(b, uint8(chunk.MessageTimeStamp>>16), uint8(chunk.MessageTimeStamp>>8), uint8(chunk.MessageTimeStamp))
		b = append(b, uint8(chunk.MessageLength>>16), uint8(chunk.MessageLength>>8), uint8(chunk.MessageLength))
		b = append(b, uint8(chunk.MessageType))
		b = append(b, 0x0, 0x0, 0x0, 0x0)
	case FMT1:
		b = append(b, uint8(chunk.MessageTimeStamp>>16), uint8(chunk.MessageTimeStamp>>8), uint8(chunk.MessageTimeStamp))
		b = append(b, uint8(chunk.MessageLength>>16), uint8(chunk.MessageLength>>8), uint8(chunk.MessageLength))
		b = append(b, uint8(chunk.MessageType))
	case FMT2:
		b = append(b, uint8(chunk.MessageTimeStamp>>16), uint8(chunk.MessageTimeStamp>>8), uint8(chunk.MessageTimeStamp))
	case FMT3:
		//XXX
	default:
		return errors.Errorf("invalid fmt_c:%d", chunk.Fmt)
	}
	return easyerrors.HandleMultiError(easyerrors.Simple(), rtmp.conn.WriteFull(b), rtmp.conn.WriteFull(chunk.Payload))
}
