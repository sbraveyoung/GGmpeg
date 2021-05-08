package rtmp

import (
	"bytes"
	"fmt"
	"io"
	"reflect"

	amf_pkg "github.com/SmartBrave/GGmpeg/rtmp/amf"
	"github.com/SmartBrave/utils/easyerrors"
	"github.com/SmartBrave/utils/easyio"
	"github.com/fatih/structs"
	"github.com/goinggo/mapstructure"
	"github.com/pkg/errors"
)

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
	MessageBase
	CommandName   string
	TranscationID int
	CommandObject ConnectReqCommandObject
}

func parseCommandMessage(rtmp *RTMP, chunk *Chunk) (cm *CommandMessage, err error) {
	var array []interface{}

	var amf amf_pkg.AMF
	amf = amf_pkg.AMF0{}
	if chunk.MessageType == COMMAND_MESSAGE_AMF3 {
		// amf= amf_pkg.AMF3{}
	}
	array, err = amf.Decode(chunk.Payload)
	if err != nil {
		return nil, errors.Wrap(err, "amf.Decode")
	}

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

func (cm *CommandMessage) Update(chunk *Chunk) error {
	if cm.messageLengthRemain != 0 {
		return nil
	}
	return nil
}

func (cm *CommandMessage) Do() (err error) {
	switch cm.CommandName {
	case CONNECT:
		err1 := NewWindowAcknowledgeSizeMessage(cm.rtmp, 2500000).Do()
		err2 := NewSetPeerBandWidthMessage(cm.rtmp, 2500000, 0x02).Do()

		// fmt.Println("333")
		// _, err = ParseChunk(conn) //ignore temporary
		// if err != nil {
		// return err
		// }

		err3 := NewUserControlMessage(cm.rtmp, StreamBegin).Do()
		err4 := NewCommandMessageResponse(cm.rtmp, _RESULT).Do()

		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4)
	case CALL:
	case CLOSE:
	case CREATE_STREAM:
	case _RESULT:
	case _ERROR:
	default:
	}

	return err
}

type ConnectRespCommandObject struct {
	FmsVer         string  `structs:"fmsVer,omitempty"`
	Level          string  `structs:"level,omitempty"`
	Code           string  `structs:"code,omitempty"`
	Description    string  `structs:"description,omitempty"`
	ObjectEncoding float64 `structs:"object_encoding,omitempty"`
}

type CommandMessageResponse struct {
	MessageBase
	CommandName   string
	TranscationID int
	CommandObject ConnectRespCommandObject
}

func NewCommandMessageResponse(rtmp *RTMP, commandName string) (cm *CommandMessageResponse) {
	return &CommandMessageResponse{
		MessageBase: MessageBase{
			rtmp: rtmp,
		},
		CommandName:   commandName,
		TranscationID: 1,
		CommandObject: ConnectRespCommandObject{
			FmsVer: "FMS/3,0,1,123",
		},
	}
}

func (cmr *CommandMessageResponse) Do() (err error) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := amf_pkg.AMF0{}

	err1 := amf.Encode(writer, cmr.CommandName)
	err2 := amf.Encode(writer, cmr.TranscationID)
	err3 := amf.Encode(writer, structs.Map(cmr.CommandObject))
	err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
	if err != nil {
		fmt.Println("HandleMultiError error:", err)
		return err
	}

	var b []byte
	b, err = io.ReadAll(buf)
	if err != nil {
		return err
	}
	return NewChunk(COMMAND_MESSAGE_AMF0, FMT0, b).Send(cmr.rtmp)
}
