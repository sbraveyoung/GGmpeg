package librtmp

import (
	"encoding/binary"
	"fmt"
)

// CSIDs used by the protocol-control path. The values here predate the
// project's adoption of the spec's reserved CSID 2 (chunk.Send refuses
// anything < 3), so we keep distinct values per message type rather
// than collapsing onto a single control channel.
const (
	csidProtocolControl uint32 = 11 //SetChunkSize, WindowAck, Acknowledgement, Abort
	csidPeerBandWidth   uint32 = 12
	csidUserControl     uint32 = 13
	csidCommand         uint32 = 10 //AMF0 command messages
	csidAudio           uint32 = 4
	csidVideo           uint32 = 9
	csidData            uint32 = 6
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
	return NewChunk(WINDOW_ACKNOWLEDGEMENT_SIZE, 4, wasm.messageTime, FMT0, csidProtocolControl, b).Send(wasm.rtmp)
}

func (wasm *WindowAcknowledgeSizeMessage) Parse() (err error) {
	wasm.AcknowledgementWindowSize = binary.BigEndian.Uint32(wasm.messagePayload)
	fmt.Println("windowAcknowledgeSize:", wasm.AcknowledgementWindowSize)
	return nil
}

func (wasm *WindowAcknowledgeSizeMessage) Do() (err error) {
	//remember the peer's window so our Acknowledgement cadence matches.
	wasm.rtmp.peerWindowAckSize = wasm.AcknowledgementWindowSize
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
	return NewChunk(SET_PEER_BANDWIDTH, 5, spbwm.messageTime, FMT0, csidPeerBandWidth, b).Send(spbwm.rtmp)
}

func (spbwm *SetPeerBandWidthMessage) Parse() (err error) {
	if len(spbwm.messagePayload) < 5 {
		return fmt.Errorf("SetPeerBandWidth payload too short: %d", len(spbwm.messagePayload))
	}
	spbwm.AcknowledgementWindowSize = binary.BigEndian.Uint32(spbwm.messagePayload[:4])
	spbwm.LimitType = LimitType(spbwm.messagePayload[4])
	fmt.Println("peerBandWidth:", spbwm.AcknowledgementWindowSize)
	fmt.Println("limitType:", spbwm.LimitType)
	return nil
}

func (spbwm *SetPeerBandWidthMessage) Do() (err error) {
	//Per RTMP 1.0 §5.4.5 the receiver MAY reply with its own
	//WindowAcknowledgementSize if the limit changed. We don't currently
	//throttle outbound traffic so simply record the value.
	spbwm.rtmp.peerBandwidth = spbwm.AcknowledgementWindowSize
	spbwm.rtmp.peerBandwidthLimit = spbwm.LimitType
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
	//All user-control events start with a 2-byte EventType. Payload
	//length depends on the event:
	// - StreamBegin / StreamEOF / StreamDry / StreamIsRecorded: 4 bytes
	//   containing the stream id
	// - SetBufferLength: 4 bytes stream id + 4 bytes buffer length
	// - PingRequest / PingResponse: 4 bytes timestamp
	var payload []byte
	switch ucm.EventType {
	case StreamBegin, StreamEOF, StreamDry, StreamIsRecorded:
		payload = make([]byte, 2+4)
		binary.BigEndian.PutUint16(payload, uint16(ucm.EventType))
		binary.BigEndian.PutUint32(payload[2:], ucm.messageStreamID)
	case SetBufferLength:
		if len(ucm.EventData) < 4 {
			return fmt.Errorf("SetBufferLength requires 4-byte buffer length in EventData")
		}
		payload = make([]byte, 2+4+4)
		binary.BigEndian.PutUint16(payload, uint16(ucm.EventType))
		binary.BigEndian.PutUint32(payload[2:], ucm.messageStreamID)
		copy(payload[6:], ucm.EventData[:4])
	case PingRequest, PingResponse:
		payload = make([]byte, 2+4)
		binary.BigEndian.PutUint16(payload, uint16(ucm.EventType))
		if len(ucm.EventData) >= 4 {
			copy(payload[2:], ucm.EventData[:4])
		}
	default:
		return fmt.Errorf("unsupported user control event type: %d", ucm.EventType)
	}
	return NewChunk(USER_CONTROL_MESSAGE, uint32(len(payload)), ucm.messageTime, FMT0, csidUserControl, payload).Send(ucm.rtmp)
}

func (ucm *UserControlMessage) Parse() (err error) {
	if len(ucm.messagePayload) < 2 {
		return fmt.Errorf("UserControlMessage payload too short: %d", len(ucm.messagePayload))
	}
	ucm.EventType = EventType(binary.BigEndian.Uint16(ucm.messagePayload[0:2]))
	ucm.EventData = ucm.messagePayload[2:]
	fmt.Println("----------ucm.EventType:", ucm.EventType)
	return nil
}

func (ucm *UserControlMessage) Do() (err error) {
	//React to ping requests so RTMP clients that rely on them for
	//keep-alive don't eventually reset the connection.
	if ucm.EventType == PingRequest && len(ucm.EventData) >= 4 {
		reply := &UserControlMessage{
			MessageBase: ucm.MessageBase,
			EventType:   PingResponse,
			EventData:   append([]byte(nil), ucm.EventData[:4]...),
		}
		return reply.Send()
	}
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
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(scsm.NewChunkSize))
	return NewChunk(SET_CHUNK_SIZE, 4, scsm.messageTime, FMT0, csidProtocolControl, b).Send(scsm.rtmp)
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
	ChunkStreamID uint32
}

func NewAbortMessage(mb MessageBase, fields ...interface{} /*ChunkStreamID uint32*/) (am *AbortMessage) {
	am = &AbortMessage{
		MessageBase: mb,
	}
	if len(fields) == 1 {
		switch v := fields[0].(type) {
		case uint32:
			am.ChunkStreamID = v
		case int:
			am.ChunkStreamID = uint32(v)
		}
	}
	return am
}

func (am *AbortMessage) Send() error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, am.ChunkStreamID)
	return NewChunk(ABORT_MESSAGE, 4, am.messageTime, FMT0, csidProtocolControl, b).Send(am.rtmp)
}

func (am *AbortMessage) Parse() (err error) {
	if len(am.messagePayload) < 4 {
		return fmt.Errorf("AbortMessage payload too short: %d", len(am.messagePayload))
	}
	am.ChunkStreamID = binary.BigEndian.Uint32(am.messagePayload[:4])
	return nil
}

func (am *AbortMessage) Do() error {
	//Drop any partial message on the indicated chunk stream so the next
	//chunk on that CSID starts a fresh message.
	delete(am.rtmp.lastChunk, am.ChunkStreamID)
	return nil
}

type AcknowledgeMessage struct {
	MessageBase
	SequenceNumber uint32
}

func NewAcknowledgeMessage(mb MessageBase, fields ...interface{} /*SequenceNumber uint32*/) (am *AcknowledgeMessage) {
	am = &AcknowledgeMessage{
		MessageBase: mb,
	}
	if len(fields) == 1 {
		switch v := fields[0].(type) {
		case uint32:
			am.SequenceNumber = v
		case int:
			am.SequenceNumber = uint32(v)
		}
	}
	return am
}

func (am *AcknowledgeMessage) Send() error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, am.SequenceNumber)
	return NewChunk(ACKNOWLEDGEMENT, 4, am.messageTime, FMT0, csidProtocolControl, b).Send(am.rtmp)
}

func (am *AcknowledgeMessage) Parse() (err error) {
	if len(am.messagePayload) < 4 {
		return fmt.Errorf("AcknowledgeMessage payload too short: %d", len(am.messagePayload))
	}
	am.SequenceNumber = binary.BigEndian.Uint32(am.messagePayload[:4])
	return nil
}

func (am *AcknowledgeMessage) Do() error {
	return nil
}
