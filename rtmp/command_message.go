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
	CONNECT        = "connect"
	RELEASE_STREAM = "releaseStream"
	FCPUBLISH      = "FCPublish"
	CALL           = "call"
	CLOSE          = "close"
	CREATE_STREAM  = "createStream"
	PUBLISH        = "publish"

	_RESULT   = "_result"
	_ERROR    = "_error"
	ON_STATUS = "onStatus"
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
	CommandName    string
	TranscationID  int
	CommandObject  ConnectReqCommandObject
	PublishingName string //releaseStream,FCPublish,publish
	//Type of publishing. Set to "live", "record", or "append".
	//TODO:record: The stream is published and the data is recorded to a new file. The file is stored on the server in a subdirectory within the directory that contains the server application. If the file already exists, it is overwritten.
	//TODO:append: The stream is published and the data is appended to a file. If no file is found, it is created.
	//live: Live data is published without recording it in a file.
	PublishingType string //publish
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

	switch cm.CommandName {
	case CONNECT:
		err = mapstructure.Decode(array[2], &cm.CommandObject)
		if err != nil {
			return nil, errors.Wrap(err, "mapstructure.Decode")
		}
	case RELEASE_STREAM, FCPUBLISH: //ignore
		cm.PublishingName = array[3].(string)
	case CREATE_STREAM: //do nothing
	case PUBLISH:
		cm.PublishingName = array[3].(string)
		cm.PublishingType = array[4].(string)
	}
	return cm, nil
}

func (cm *CommandMessage) Update(chunk *Chunk) error {
	newCm, err := parseCommandMessage(cm.rtmp, chunk)
	if err != nil {
		return err
	}
	cm.CommandName = newCm.CommandName
	cm.TranscationID = newCm.TranscationID
	cm.CommandObject = newCm.CommandObject
	return nil
}

func (cm *CommandMessage) Do() (err error) {
	switch cm.CommandName {
	case CONNECT:
		err1 := NewWindowAcknowledgeSizeMessage(cm.rtmp, 2500000).Do()
		err2 := NewSetPeerBandWidthMessage(cm.rtmp, 2500000, 0x02).Do()
		err3 := NewUserControlMessage(cm.rtmp, StreamBegin).Do()
		err4 := NewCommandMessageResponse(cm.rtmp, cm.CommandName, _RESULT, cm.TranscationID, 0).Do()
		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4)
	case RELEASE_STREAM, FCPUBLISH: //ignore
	case CALL:
	case CLOSE:
	case CREATE_STREAM:
		err = NewCommandMessageResponse(cm.rtmp, cm.CommandName, _RESULT, cm.TranscationID, cm.messageStreamID).Do()
	case PUBLISH:
		err = NewCommandMessageResponse(cm.rtmp, cm.CommandName, ON_STATUS, cm.TranscationID, 0).Do()
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
	CommandName     string
	CommandRespName string
	TranscationID   int
	CommandObject   ConnectRespCommandObject
	StreamID        uint32
}

func NewCommandMessageResponse(rtmp *RTMP, commandName, commandRespName string, transcationID int, streamID uint32) (cm *CommandMessageResponse) {
	cmr := &CommandMessageResponse{
		MessageBase: MessageBase{
			rtmp: rtmp,
		},
		CommandName:     commandName,
		CommandRespName: commandRespName,
		TranscationID:   transcationID,
	}

	switch commandName {
	case CONNECT:
		cmr.CommandObject.FmsVer = "FMS/3,0,1,123"
	case CREATE_STREAM:
		cmr.StreamID = streamID
	case PUBLISH:
		cmr.CommandObject.Level = "status"
		cmr.CommandObject.Code = "NetStream.Publish.Start"
		cmr.CommandObject.Description = "publishing"
	default: //do nothing
	}
	return cmr
}

func (cmr *CommandMessageResponse) Do() (err error) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := amf_pkg.AMF0{}

	var err1, err2, err3, err4 error
	err1 = amf.Encode(writer, cmr.CommandRespName)
	err2 = amf.Encode(writer, cmr.TranscationID)
	switch cmr.CommandName {
	case CONNECT:
		err3 = amf.Encode(writer, structs.Map(cmr.CommandObject))
	case CREATE_STREAM:
		err3 = amf.Encode(writer, nil)
		err4 = amf.Encode(writer, cmr.StreamID)
	case PUBLISH:
		err3 = amf.Encode(writer, nil)
		err4 = amf.Encode(writer, structs.Map(cmr.CommandObject))
	}
	err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4)
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
