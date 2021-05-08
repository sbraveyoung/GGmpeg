package rtmp

import "encoding/binary"

type WindowAcknowledgeSizeMessage struct {
	MessageBase
	AcknowledgementWindowSize uint32
}

func NewWindowAcknowledgeSizeMessage(rtmp *RTMP, windowSize uint32) (wasm *WindowAcknowledgeSizeMessage) {
	return &WindowAcknowledgeSizeMessage{
		MessageBase: MessageBase{
			rtmp: rtmp,
		},
		AcknowledgementWindowSize: windowSize,
	}
}

func (wasm *WindowAcknowledgeSizeMessage) Do() (err error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, wasm.AcknowledgementWindowSize)
	return NewChunk(WINDOW_ACKNOWLEDGEMENT_SIZE, FMT0, b).Send(wasm.rtmp)
}

type SetPeerBandWidthMessage struct {
	MessageBase
	AcknowledgementWindowSize uint32
	LimitType                 uint8
}

func NewSetPeerBandWidthMessage(rtmp *RTMP, windowSize uint32, limitType uint8) (spbwm *SetPeerBandWidthMessage) {
	return &SetPeerBandWidthMessage{
		MessageBase: MessageBase{
			rtmp: rtmp,
		},
		AcknowledgementWindowSize: windowSize,
		LimitType:                 limitType,
	}
}

func (spbwm *SetPeerBandWidthMessage) Do() (err error) {
	b := make([]byte, 5)
	binary.BigEndian.PutUint32(b, spbwm.AcknowledgementWindowSize)
	b[4] = byte(spbwm.LimitType)
	return NewChunk(SET_PEER_BANDWIDTH, FMT0, b).Send(spbwm.rtmp)
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

func NewUserControlMessage(rtmp *RTMP, eventType EventType) (ucm *UserControlMessage) {
	return &UserControlMessage{
		MessageBase: MessageBase{
			rtmp: rtmp,
		},
		EventType: eventType,
	}
}

func (ucm *UserControlMessage) Do() (err error) {
	var b []byte
	switch ucm.EventType {
	case StreamBegin:
		b = make([]byte, 4+2)
		binary.BigEndian.PutUint16(b, uint16(ucm.EventType))
		binary.BigEndian.PutUint32(b[2:], ucm.messageStreamID)
		return NewChunk(USER_CONTROL_MESSAGE, FMT0, b).Send(ucm.rtmp)
	case StreamEOF:
	case StreamDry:
	case SetBufferLength:
	case PingRequest:
	case PingResponse:
	default:
	}
	return nil
}

// type SetChunkSizeMessage struct {
// MessageBase
// MessageHeader
// NewChunkSize uint32
// }

// func newSetChunkSizeMessage(rtmp *RTMP, chunk *Chunk) (scsm *SetChunkSizeMessage, err error) {
// scsm = &SetChunkSizeMessage{
// MessageBase: MessageBase{
// rtmp: rtmp,
// },
// MessageHeader: MessageHeader{
// // MessageType:
// // PayloadLength:
// // TimeStamp:
// // MessageStreamID:
// },
// // NewChunkSize:
// }
// }

// func (mscs *SetChunkSizeMessage) Combine(chunk *Chunk) error {
// }

// func (mscs *SetChunkSizeMessage) Do() error {
// //TODO
// return nil
// }

// type MessageAbort struct {
// MessageHeader
// ChunkStreamID uint32
// }

// func (ma *MessageAbort) Do() error {
// //TODO
// return nil
// }

// type MessageAcknowledgement struct {
// MessageHeader
// SequenceNumber uint32
// }

// func (ma *MessageAcknowledgement) Do() error {
// //TODO
// return nil
// }

// type MessageUserControl struct {
// MessageHeader
// EventType uint16
// EventData []byte
// }

// func (muc *MessageUserControl) Do() error {
// //TODO
// return nil
// }

// type LimitType uint8

// const (
// HARD    LimitType = 0
// SOFT    LimitType = 1
// DYNAMIC LimitType = 2
// )

// type MessageSetPeerBandwidth struct {
// MessageHeader
// AcknowledgementWindowSize uint32
// LimitType                 LimitType
// }

// func (mspb *MessageSetPeerBandwidth) Do() error {
// //TODO
// return nil
// }
