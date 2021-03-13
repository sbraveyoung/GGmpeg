package rtmp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"

	"github.com/goinggo/mapstructure"
	"github.com/gwuhaolin/livego/protocol/amf"
	"github.com/pkg/errors"
)

type Message interface {
	Combine(*Chunk) error
	Do(rtmpConn) error
}
type MessageBase struct {
	rtmp *RTMP
}
type MessageHeader struct {
	MessageType     uint8
	PayloadLength   uint32 //3bytes
	TimeStamp       uint32
	MessageStreamID uint32 //3bytes
}

/*
 * MessageType 1-7
   --- control message ---
     -- MessageStreamID: 0
     -- ChunkStreamID: 2
	 -- TimeStamp: ignore
   * 1: set chunk size
   * 2: abort message
   * 3: acknowledgement
   * 4: user control message
   * 5: window acknowledgement size
   * 6: set peer bandwidth
   * 7: TODO is used between edge server and origin server
   --- common message ---
   * 8: audio message
   * 9: video message
   --- command message ---
   * 15(AMF3)/18(AMF0): data message
   * 16(AMF3)/19(AMF0): share object message
   * 17(AMF3)/20(AMF0): command message
   * 22: aggregate message
*/
type MessageType uint8

const (
	_ MessageType = iota
	//control message
	SET_CHUNK_SIZE
	ABORT_MESSAGE
	ACKNOWLEDGEMENT
	USER_CONTROL_MESSAGE
	WINDOW_ACKNOWLEDGEMENT_SIZE
	SET_PEER_BANDWIDTH
	XXX

	//common message
	AUDIO_MESSAGE
	VIDEO_MESSAGE

	_
	_
	_
	_
	_
	//command message
	DATA_MESSAGE_AMF3
	SHARE_OBJECT_MESSAGE_AMF3
	COMMAND_MESSAGE_AMF3
	DATA_MESSAGE_AMF0
	SHARE_OBJECT_MESSAGE_AMF0
	COMMAND_MESSAGE_AMF0
)

func ParseMessage(rtmp *RTMP, chunk *Chunk) (message Message, err error) {
	if chunk.Fmt != FMT0 {
		return nil, errors.New("invalid chunk type")
	}

	switch chunk.MessageType {
	case SET_CHUNK_SIZE:
		// return newSetChunkSizeMessage(rtmp, chunk)
	case ABORT_MESSAGE:
	case ACKNOWLEDGEMENT:
	case USER_CONTROL_MESSAGE:
	case WINDOW_ACKNOWLEDGEMENT_SIZE:
	case SET_PEER_BANDWIDTH:
	// case XXX:

	case AUDIO_MESSAGE:
	case VIDEO_MESSAGE:

	case DATA_MESSAGE_AMF0, DATA_MESSAGE_AMF3:
	case SHARE_OBJECT_MESSAGE_AMF0, SHARE_OBJECT_MESSAGE_AMF3:
	case COMMAND_MESSAGE_AMF0, COMMAND_MESSAGE_AMF3:
		return parseCommandMessage(rtmp, chunk)
	default:
		//do nothing
	}
	return nil, errors.New("invalue message type")
}

type CommandName string

const (
	CONNECT       CommandName = "connect"
	CALL                      = "call"
	CLOSE                     = "close"
	CREATE_STREAM             = "createStream"
)

type CommandObject struct {
	App            string  `mapstructure:"app"`
	FlashVer       string  `mapstructure:"flashver"`
	SwfURL         string  `mapstructure:"swfUrl"`
	TcURL          string  `mapstructure:"tcUrl"`
	Fpad           bool    `mapstructure:"fpad"`
	AudioCodecs    float64 `mapstructure:"audioCodecs"`
	VideoCodecs    float64 `mapstructure:"videoCodecs"`
	VideoFunction  float64 `mapstructure:"videoFunction"`
	PageURL        string  `mapstructure:"pageUrl"`
	ObjectEncoding float64 `mapstructure:"objectEncoding"`
}

type CommandMessage struct {
	CommandName   CommandName
	TranscationID int
	CommandObject CommandObject
}

func parseCommandMessage(rtmp *RTMP, chunk *Chunk) (cm *CommandMessage, err error) {
	r := bytes.NewBuffer(chunk.Payload)
	amfDecoder := amf.NewDecoder()
	v := amf.Version(amf.AMF0)
	if chunk.MessageType == COMMAND_MESSAGE_AMF3 {
		v = amf.AMF3
	}
	var array []interface{}
	array, err = amfDecoder.DecodeBatch(r, v)
	if err != nil && err != io.EOF {
		return nil, errors.Wrap(err, "amfDecoder.Decode")
	}
	if len(array) < 3 {
		return nil, errors.New("invalid data")
	}
	for index, a := range array {
		fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	}
	cm = &CommandMessage{
		CommandName:   CommandName(array[0].(string)),
		TranscationID: int(array[1].(float64)),
	}
	if cm.TranscationID != 1 {
		return nil, errors.New("invalid transcation id")
	}
	err = mapstructure.Decode(array[2], &cm.CommandObject)
	if err != nil {
		return nil, errors.Wrap(err, "mapstructure.Decode")
	}
	fmt.Printf("command message struct:%+v\n", *cm)
	return cm, nil
}

func (cm *CommandMessage) Combine(chunk *Chunk) error {
	return nil
}

func (cm *CommandMessage) Do(conn rtmpConn) error {
	chunk := NewChunk(WINDOW_ACKNOWLEDGEMENT_SIZE, NewWindowAcknowledgeSizeMessage())
	b := make([]byte, 0, 11+len(chunk.Payload))
	b = append(b, byte(uint8(chunk.Fmt<<6)|uint8(chunk.CsID&0x3f)))
	b = append(b, uint8(chunk.MessageTimeStampDelta>>16), uint8(chunk.MessageTimeStampDelta>>8), uint8(chunk.MessageTimeStampDelta))
	b = append(b, uint8(chunk.MessageLength>>16), uint8(chunk.MessageLength>>8), uint8(chunk.MessageLength))
	b = append(b, uint8(chunk.MessageType))
	b = append(b, 0x0, 0x0, 0x0, 0x0)
	b = append(b, chunk.Payload...)
	return conn.Write(b)
}

type WindowAcknowledgeSizeMessage struct {
	AcknowledgementWindowSize uint32
}

func NewWindowAcknowledgeSizeMessage() []byte {
	wasm := &WindowAcknowledgeSizeMessage{
		AcknowledgementWindowSize: 2500000,
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, wasm.AcknowledgementWindowSize)
	return b
}

// type ControlMessage interface {
// Combine(*Chunk) error
// Do() error
// }

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
