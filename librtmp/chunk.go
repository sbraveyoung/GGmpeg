package librtmp

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
	Fmt  MessageHeaderType //2 bits
	CsID uint32            //6, 14 or 22 bits
}

func parseChunkBasicHeader(rtmp *RTMP) (cbhp *ChunkBasicHeader, err error) {
	b, err := rtmp.readerConn.ReadN(1)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("basic header:%x\n", b)

	cbhp = &ChunkBasicHeader{
		Fmt: MessageHeaderType((b[0] & 0xc0) >> 6),
	}

	switch csid := b[0] & 0x3f; csid {
	case 0x0:
		b1, err := rtmp.readerConn.ReadN(1)
		if err != nil {
			return nil, err
		}
		cbhp.CsID = uint32(b1[0]) + 64
	case 0x1:
		b2, err := rtmp.readerConn.ReadN(2)
		if err != nil {
			return nil, err
		}
		cbhp.CsID = uint32(b2[0]) + uint32(b2[1])*256 + 64
	case 0x2:
		//XXX
	default:
		cbhp.CsID = uint32(csid)
	}
	// fmt.Printf("basic header struct:%+v\n", *cbhp)
	return cbhp, nil
}

type ChunkMessageHeader struct {
	MessageTimeStamp uint32 //3bytes or 4bytes(extended timestamp)
	MessageTimeDelta uint32
	MessageLength    uint32 //3bytes
	MessageType      MessageType
	MessageStreamID  uint32 //little-endian 4bytes
}

func parseChunkMessageHeader(rtmp *RTMP, basicHeader *ChunkBasicHeader, firstChunkinMessage bool) (cmhp *ChunkMessageHeader, err error) {
	if basicHeader.Fmt != FMT0 && rtmp.lastChunk[basicHeader.CsID] == nil {
		return nil, errors.Errorf("invalid fmt_a:%d", basicHeader.Fmt)
	}

	cmhp = &ChunkMessageHeader{}
	switch basicHeader.Fmt {
	case FMT0:
		b11, err := rtmp.readerConn.ReadN(11)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = uint32(0x00)<<24 | uint32(b11[0])<<16 | uint32(b11[1])<<8 | uint32(b11[2])
		cmhp.MessageTimeDelta = 0
		cmhp.MessageLength = uint32(0x00)<<24 | uint32(b11[3])<<16 | uint32(b11[4])<<8 | uint32(b11[5])
		cmhp.MessageType = MessageType(b11[6])
		cmhp.MessageStreamID = binary.LittleEndian.Uint32(b11[7:])
	case FMT1:
		b7, err := rtmp.readerConn.ReadN(7)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = rtmp.lastChunk[basicHeader.CsID].MessageTimeStamp
		cmhp.MessageTimeDelta = uint32(0x00)<<24 | uint32(b7[0])<<16 | uint32(b7[1])<<8 | uint32(b7[2])
		cmhp.MessageTimeStamp += cmhp.MessageTimeDelta
		cmhp.MessageLength = uint32(0x00)<<24 | uint32(b7[3])<<16 | uint32(b7[4])<<8 | uint32(b7[5])
		cmhp.MessageType = MessageType(b7[6])
		cmhp.MessageStreamID = rtmp.lastChunk[basicHeader.CsID].MessageStreamID
	case FMT2:
		b3, err := rtmp.readerConn.ReadN(3)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = rtmp.lastChunk[basicHeader.CsID].MessageTimeStamp
		cmhp.MessageTimeDelta = uint32(0x00)<<24 | uint32(b3[0])<<16 | uint32(b3[1])<<8 | uint32(b3[2])
		cmhp.MessageTimeStamp += cmhp.MessageTimeDelta
		cmhp.MessageLength = rtmp.lastChunk[basicHeader.CsID].MessageLength
		cmhp.MessageType = rtmp.lastChunk[basicHeader.CsID].MessageType
		cmhp.MessageStreamID = rtmp.lastChunk[basicHeader.CsID].MessageStreamID
	case FMT3:
		cmhp.MessageTimeStamp = rtmp.lastChunk[basicHeader.CsID].MessageTimeStamp
		cmhp.MessageTimeDelta = rtmp.lastChunk[basicHeader.CsID].MessageTimeDelta
		cmhp.MessageLength = rtmp.lastChunk[basicHeader.CsID].MessageLength
		cmhp.MessageType = rtmp.lastChunk[basicHeader.CsID].MessageType
		cmhp.MessageStreamID = rtmp.lastChunk[basicHeader.CsID].MessageStreamID
		//NOTE: 2 cases with FMT3
		//1. A single message is split into chunks, all chunks of a message except the first one SHOULD use this type.
		//2. A stream consisting of messages of exactly the same size, stream ID and spacing in time SHOULD use this type for all chunks after a chunk of Type 2.
		if firstChunkinMessage {
			cmhp.MessageTimeStamp += cmhp.MessageTimeDelta
		} else {
		}
	default:
		return nil, errors.Errorf("invalid fmt_b:%d", basicHeader.Fmt)
	}
	if cmhp.MessageTimeStamp == 0xffffff {
		b4, err := rtmp.readerConn.ReadN(4)
		if err != nil {
			return nil, err
		}
		cmhp.MessageTimeStamp = binary.BigEndian.Uint32(b4)
	}
	fmt.Printf("[message] chunk fmt:%d, csid:%d, firstChuninMessage:%t, messageType:%d, timeStampDelta:%d, timeStamp:%d\n", basicHeader.Fmt, basicHeader.CsID, firstChunkinMessage, cmhp.MessageType, cmhp.MessageTimeDelta, cmhp.MessageTimeStamp)
	// fmt.Printf("message header struct:%+v\n", *cmhp)
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

	messageHeader, err := parseChunkMessageHeader(rtmp, basicHeader, message == nil)
	if err != nil {
		return nil, err
	}

	chunkSize := messageHeader.MessageLength
	if chunkSize > rtmp.peerMaxChunkSize {
		chunkSize = rtmp.peerMaxChunkSize
	}
	if message != nil {
		if remain := message.Remain(); chunkSize > remain {
			chunkSize = remain
		}
	}
	b := make([]byte, chunkSize)
	err = rtmp.readerConn.ReadFull(b)
	if err != nil {
		return nil, err
	}
	cp = &Chunk{
		ChunkBasicHeader:   *basicHeader,
		ChunkMessageHeader: *messageHeader,
		Payload:            b,
	}

	rtmp.lastChunk[cp.CsID] = cp
	return cp, nil
}

//NOTE: ensure len(payload) <= peerMaxChunkSize
func NewChunk(messageType MessageType, messageLength uint32, messageTime uint32, format MessageHeaderType, csid uint32, payload []byte) (chunk *Chunk) {
	return &Chunk{
		ChunkBasicHeader: ChunkBasicHeader{
			Fmt:  format,
			CsID: csid,
		},
		ChunkMessageHeader: ChunkMessageHeader{
			MessageTimeStamp: messageTime,
			MessageLength:    messageLength,
			MessageType:      messageType,
			MessageStreamID:  0,
		},
		Payload: payload,
	}
}

func (chunk *Chunk) Send(rtmp *RTMP) (err error) {
	b := []byte{}
	if chunk.CsID < 3 {
		return errors.New("invalid csid")
	} else if chunk.CsID < 64 {
		b = append(b, byte(uint8(chunk.Fmt<<6)|uint8(chunk.CsID&0x3f)))
	} else if chunk.CsID < 320 {
		b = append(b, uint8(chunk.Fmt<<6))
		b = append(b, uint8(chunk.CsID-64))
	} else {
		b = append(b, uint8(chunk.Fmt<<6)|uint8(0x01))
		b = append(b, uint8(0), uint8(0))
		binary.BigEndian.PutUint16(b[len(b)-2:], uint16(chunk.CsID))
	}
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
	return rtmp.writerConn.WriteFull(append(b, chunk.Payload...))
}
