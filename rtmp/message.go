package rtmp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"

	// amf1 "github.com/gwuhaolin/livego/protocol/amf"
	amf_pkg "github.com/SmartBrave/GGmpeg/rtmp/amf"
	"github.com/SmartBrave/utils/easyerrors"
	"github.com/SmartBrave/utils/easyio"
	"github.com/fatih/structs"
	"github.com/goinggo/mapstructure"
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

const (
	CONNECT       = "connect"
	CALL          = "call"
	CLOSE         = "close"
	CREATE_STREAM = "createStream"

	_RESULT = "_result"
	_ERROR  = "_error"
)

type ConnectReqCommandObject struct {
	App            string        `mapstructure:"app"`
	FlashVer       string        `mapstructure:"flashver"`
	SwfURL         string        `mapstructure:"swfUrl"`
	TcURL          string        `mapstructure:"tcUrl"`
	Fpad           bool          `mapstructure:"fpad"`
	AudioCodecs    AudioCodec    `mapstructure:"audioCodecs"`
	VideoCodecs    VideoCodec    `mapstructure:"videoCodecs"`
	VideoFunction  VideoFunction `mapstructure:"videoFunction"`
	PageURL        string        `mapstructure:"pageUrl"`
	ObjectEncoding float64       `mapstructure:"objectEncoding"`
	Type           string        `mapstructure:"type"`
}

type CommandMessage struct {
	CommandName   string
	TranscationID int
	CommandObject ConnectReqCommandObject
}

type ConnectRespCommandObject struct {
	FmsVer         string  `structs:"fmsVer,omitempty"`
	Level          string  `structs:"level,omitempty"`
	Code           string  `structs:"code,omitempty"`
	Description    string  `structs:"description,omitempty"`
	ObjectEncoding float64 `structs:"object_encoding,omitempty"`
}

type CommandMessageResponse struct {
	CommandName   string
	TranscationID int
	CommandObject ConnectRespCommandObject
}

func NewCommandMessage(commandName string) (b []byte) {
	resp := &CommandMessageResponse{
		CommandName:   commandName,
		TranscationID: 1,
		CommandObject: ConnectRespCommandObject{
			FmsVer: "FMS/3,0,1,123",
		},
	}
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewWriter(buf)
	amf := amf_pkg.AMF0{}

	err1 := amf.Encode(writer, resp.CommandName)
	err2 := amf.Encode(writer, resp.TranscationID)
	err3 := amf.Encode(writer, structs.Map(resp.CommandObject))
	err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
	if err != nil {
		fmt.Println("err:", err)
		return buf.Bytes()
	}

	b, err = io.ReadAll(buf)
	if err != nil {
		return buf.Bytes()
	}
	// fmt.Printf("resp data:%x", b)
	return b
}

func parseCommandMessage(rtmp *RTMP, chunk *Chunk) (cm *CommandMessage, err error) {
	var array []interface{}
	r := easyio.NewReader(bytes.NewReader(chunk.Payload))

	var amf amf_pkg.AMF
	amf = amf_pkg.AMF0{}
	if chunk.MessageType == COMMAND_MESSAGE_AMF3 {
		// amf= amf_pkg.AMF3{}
	}
	array, err = amf.Decode(r)
	if err != nil {
		return nil, errors.Wrap(err, "amf.Decode")
	}

	// amfDecoder := amf1.NewDecoder()
	// v := amf1.Version(amf1.AMF0)
	// array, err = amfDecoder.DecodeBatch(r, v)
	// if err != nil && err != stdio.EOF {
	// return nil, errors.Wrap(err, "amfDecoder.Decode")
	// }

	if len(array) < 3 {
		return nil, errors.New("invalid data")
	}
	for index, a := range array {
		fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	}
	cm = &CommandMessage{
		CommandName:   array[0].(string),
		TranscationID: int(array[1].(float64)),
	}
	if cm.TranscationID != 1 {
		return nil, errors.New("invalid transcation id")
	}
	err = mapstructure.Decode(array[2], &cm.CommandObject)
	if err != nil {
		return nil, errors.Wrap(err, "mapstructure.Decode")
	}
	return cm, nil
}

func (cm *CommandMessage) Combine(chunk *Chunk) error {
	return nil
}

func (cm *CommandMessage) Do(conn rtmpConn) (err error) {
	switch cm.CommandName {
	case CONNECT:
		chunk := NewChunk(WINDOW_ACKNOWLEDGEMENT_SIZE, NewWindowAcknowledgeSizeMessage(2500000))
		err1 := chunk.Send(conn)

		chunk = NewChunk(SET_PEER_BANDWIDTH, NewSetPeerBandWidthMessage(2500000, 0x02))
		err2 := chunk.Send(conn)

		// fmt.Println("333")
		// _, err = ParseChunk(conn) //ignore temporary
		// if err != nil {
		// return err
		// }

		chunk = NewChunk(USER_CONTROL_MESSAGE, NewUserControlMessage(StreamBegin))
		err3 := chunk.Send(conn)

		chunk = NewChunk(COMMAND_MESSAGE_AMF0, NewCommandMessage(_RESULT))
		err4 := chunk.Send(conn)
		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4)
	case CALL:
	case CLOSE:
	case CREATE_STREAM:
	default:
	}

	return nil
}

type WindowAcknowledgeSizeMessage struct {
	AcknowledgementWindowSize uint32
}

func NewWindowAcknowledgeSizeMessage(windowSize uint32) []byte {
	wasm := &WindowAcknowledgeSizeMessage{
		AcknowledgementWindowSize: windowSize,
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, wasm.AcknowledgementWindowSize)
	return b
}

type SetPeerBandWidthMessage struct {
	AcknowledgementWindowSize uint32
	LimitType                 uint8
}

func NewSetPeerBandWidthMessage(windowSize uint32, limitType uint8) []byte {
	spbwm := &SetPeerBandWidthMessage{
		AcknowledgementWindowSize: windowSize,
		LimitType:                 limitType,
	}
	b := make([]byte, 5)
	binary.BigEndian.PutUint32(b, spbwm.AcknowledgementWindowSize)
	b[4] = byte(spbwm.LimitType)
	return b
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
	EventType EventType
	EventData []byte
}

func NewUserControlMessage(eventType EventType) []byte {
	ucm := &UserControlMessage{
		EventType: eventType,
	}
	var b []byte
	switch eventType {
	case StreamBegin:
		ucm.EventData = make([]byte, 4, 4)
		b = make([]byte, 4+2)
		binary.BigEndian.PutUint16(b, uint16(ucm.EventType))
		copy(b[2:], ucm.EventData)
	case StreamEOF:
	case StreamDry:
	case SetBufferLength:
	case PingRequest:
	case PingResponse:
	default:
	}
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
