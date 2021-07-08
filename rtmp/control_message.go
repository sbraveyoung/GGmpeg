package rtmp

import (
	"encoding/binary"
	"fmt"
)

type WindowAcknowledgeSizeMessage struct {
	MessageBase
	AcknowledgementWindowSize uint32
}

func NewWindowAcknowledgeSizeMessage(mb MessageBase, fields ...interface{} /*windowSize int*/) (wasm *WindowAcknowledgeSizeMessage) {
	wasm = &WindowAcknowledgeSizeMessage{
		MessageBase: mb,
	}
	if len(fields) == 1 {
		var ok bool
		if wasm.AcknowledgementWindowSize, ok = fields[0].(uint32); !ok {
			wasm.AcknowledgementWindowSize = 0
		}
	}
	return wasm
}

func (wasm *WindowAcknowledgeSizeMessage) Send() (err error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(wasm.AcknowledgementWindowSize))
	return NewChunk(WINDOW_ACKNOWLEDGEMENT_SIZE, FMT0, 11, b).Send(wasm.rtmp)
}

func (wasm *WindowAcknowledgeSizeMessage) Parse() (err error) {
	wasm.AcknowledgementWindowSize = binary.BigEndian.Uint32(wasm.messagePayload)
	fmt.Println("windowAcknowledgeSize:", wasm.AcknowledgementWindowSize)
	return nil
}

func (wasm *WindowAcknowledgeSizeMessage) Do() (err error) {
	//TODO
	return nil
}

type LimitType uint8

const (
	HARD    LimitType = 0
	SOFT    LimitType = 1
	DYNAMIC LimitType = 2
)

type SetPeerBandWidthMessage struct {
	MessageBase
	AcknowledgementWindowSize uint32
	LimitType                 LimitType
}

func NewSetPeerBandWidthMessage(mb MessageBase, fields ...interface{} /*windowSize int, limitType uint8*/) (spbwm *SetPeerBandWidthMessage) {
	spbwm = &SetPeerBandWidthMessage{
		MessageBase: mb,
	}
	if len(fields) == 2 {
		var ok bool
		if spbwm.AcknowledgementWindowSize, ok = fields[0].(uint32); !ok {
			spbwm.AcknowledgementWindowSize = 0
		}
		if spbwm.LimitType, ok = fields[1].(LimitType); !ok {
			spbwm.LimitType = 0
		}
	}
	return spbwm
}

func (spbwm *SetPeerBandWidthMessage) Send() (err error) {
	b := make([]byte, 5)
	binary.BigEndian.PutUint32(b, spbwm.AcknowledgementWindowSize)
	b[4] = byte(spbwm.LimitType)
	return NewChunk(SET_PEER_BANDWIDTH, FMT0, 12, b).Send(spbwm.rtmp)
}

func (spbwm *SetPeerBandWidthMessage) Parse() (err error) {
	_ = spbwm.messagePayload[4]
	spbwm.AcknowledgementWindowSize = binary.BigEndian.Uint32(spbwm.messagePayload[:4])
	spbwm.LimitType = LimitType(spbwm.messagePayload[4])
	fmt.Println("peerBandWidth:", spbwm.AcknowledgementWindowSize)
	fmt.Println("limitType:", spbwm.LimitType)
	return nil
}

func (spbwm *SetPeerBandWidthMessage) Do() (err error) {
	//TODO
	return nil
}

type EventType uint16

const (
	StreamBegin EventType = iota
	StreamEOF
	StreamDry
	SetBufferLength
	StreamIsRecorded
	_
	PingRequest
	PingResponse
)

type UserControlMessage struct {
	MessageBase
	EventType EventType
	EventData []byte
}

func NewUserControlMessage(mb MessageBase, fields ...interface{} /*eventType EventType*/) (ucm *UserControlMessage) {
	ucm = &UserControlMessage{
		MessageBase: mb,
		// EventType:   eventType,
	}
	var ok bool
	switch len(fields) {
	case 2:
		if ucm.EventData, ok = fields[1].([]byte); !ok {
			ucm.EventData = []byte{}
		}
		fallthrough
	case 1:
		if ucm.EventType, ok = fields[0].(EventType); !ok {
			ucm.EventType = StreamEOF
		}
	}
	return ucm
}

func (ucm *UserControlMessage) Send() (err error) {
	var b []byte
	switch ucm.EventType {
	case StreamBegin:
		b = make([]byte, 4+2)
		binary.BigEndian.PutUint16(b, uint16(ucm.EventType))
		binary.BigEndian.PutUint32(b[2:], ucm.messageStreamID)
		return NewChunk(USER_CONTROL_MESSAGE, FMT0, 13, b).Send(ucm.rtmp)
	case StreamEOF:
	case StreamDry:
	case SetBufferLength:
	case PingRequest:
	case PingResponse:
	default:
	}
	return nil
}

func (ucm *UserControlMessage) Parse() (err error) {
	ucm.EventType = EventType(binary.BigEndian.Uint16(ucm.messagePayload[0:2]))
	fmt.Println("----------ucm.EventType:", ucm.EventType)
	switch ucm.EventType {
	case StreamBegin:
	case StreamEOF:
	case StreamDry:
	case SetBufferLength:
	case StreamIsRecorded:
	case PingRequest:
	case PingResponse:
	default:
	}
	//TODO
	return nil
}

func (ucm *UserControlMessage) Do() (err error) {
	//TODO
	return nil
}

type SetChunkSizeMessage struct {
	MessageBase
	NewChunkSize uint32
}

func NewSetChunkSizeMessage(mb MessageBase, fields ...interface{} /*NewChunkSize int*/) (scsm *SetChunkSizeMessage) {
	scsm = &SetChunkSizeMessage{
		MessageBase: mb,
	}
	var ok bool
	if len(fields) == 1 {
		if scsm.NewChunkSize, ok = fields[0].(uint32); !ok {
			scsm.NewChunkSize = 128
		}
	}
	return scsm
}

func (scsm *SetChunkSizeMessage) Send() error {
	//TODO
	return nil
}

func (scsm *SetChunkSizeMessage) Parse() (err error) {
	scsm.NewChunkSize = binary.BigEndian.Uint32(scsm.messagePayload)
	fmt.Println("new chunk size of peer::", scsm.NewChunkSize)
	return nil
}

func (scsm *SetChunkSizeMessage) Do() error {
	scsm.rtmp.peerMaxChunkSize = scsm.NewChunkSize
	return nil
}

type AbortMessage struct {
	MessageBase
	ChunkStreamID int
}

func NewAbortMessage(mb MessageBase, fields ...interface{} /*ChunkStreamID int*/) (am *AbortMessage) {
	am = &AbortMessage{
		MessageBase: mb,
	}
	var ok bool
	if len(fields) == 1 {
		if am.ChunkStreamID, ok = fields[0].(int); !ok {
			am.ChunkStreamID = 0
		}
	}
	return am
}

func (scsm *AbortMessage) Send() error {
	//TODO
	return nil
}

func (scsm *AbortMessage) Parse() (err error) {
	//TODO
	return nil
}

func (scsm *AbortMessage) Do() error {
	//TODO
	return nil
}

type AcknowledgeMessage struct {
	MessageBase
	SequenceNumber int
}

func NewAcknowledgeMessage(mb MessageBase, fields ...interface{} /*ChunkStreamID int*/) (am *AcknowledgeMessage) {
	am = &AcknowledgeMessage{
		MessageBase: mb,
	}
	var ok bool
	if len(fields) == 1 {
		if am.SequenceNumber, ok = fields[0].(int); !ok {
			am.SequenceNumber = 0
		}
	}
	return am
}

func (scsm *AcknowledgeMessage) Send() error {
	//TODO
	return nil
}

func (scsm *AcknowledgeMessage) Parse() (err error) {
	//TODO
	return nil
}

func (scsm *AcknowledgeMessage) Do() error {
	//TODO
	return nil
}
