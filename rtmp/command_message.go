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
	FCUNPUBLISH    = "FCUnpublish"
	CALL           = "call"
	CLOSE          = "close"
	CREATE_STREAM  = "createStream"
	PUBLISH        = "publish"
	PLAY           = "play"
	PLAY2          = "play2"
	DELETE_STREAM  = "deleteStream"
	CLOSE_STREAM   = "closeStream"
	RECEIVE_AUDIO  = "receiveAudio"
	RECEIVE_VIDEO  = "receiveVideo"
	SEEK           = "seek"
	PAUSE          = "pause"

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
	Capabilities   float64       `mapstructure:"capabilities"`
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
	PublishingType string  //publish
	StreamName     string  //play
	Start          float64 //play
	Duration       float64 //play
	Reset          bool    //play
}

func NewCommandMessage(mb MessageBase, fields ...interface{} /*commandName string, transcationID int, others*/) (cm *CommandMessage) {
	cm = &CommandMessage{
		MessageBase: mb,
	}
	if len(fields) >= 2 {
		var ok bool
		if cm.CommandName, ok = fields[0].(string); !ok {
			cm.CommandName = ""
		}
		if cm.TranscationID, ok = fields[1].(int); !ok {
			cm.TranscationID = 0
		}
		//TODO: others
	}
	return cm
}

func (cm *CommandMessage) Send() (err error) {
	//TODO
	return nil
}

func (cm *CommandMessage) Parse() (err error) {
	var array []interface{}
	array, err = cm.amf.Decode(easyio.NewEasyReader(bytes.NewReader(cm.messagePayload)))
	if err != nil {
		return errors.Wrap(err, "amf.Decode")
	}

	if len(array) < 3 {
		return errors.New("invalid data")
	}
	for index, a := range array {
		fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	}

	cm.CommandName = array[0].(string)
	cm.TranscationID = int(array[1].(float64))
	switch cm.CommandName {
	case CONNECT:
		_ = array[2]
		err = mapstructure.Decode(array[2], &cm.CommandObject)
		if err != nil {
			return errors.Wrap(err, "mapstructure.Decode")
		}
	case RELEASE_STREAM, FCPUBLISH: //ignore
		_ = array[3]
		cm.PublishingName = array[3].(string)
	case FCUNPUBLISH:
	case DELETE_STREAM:
	case CREATE_STREAM: //do nothing
	case PUBLISH:
		_ = array[4]
		cm.PublishingName = array[3].(string)
		cm.PublishingType = array[4].(string)
	case PLAY:
		cm.PublishingName = array[3].(string)
	}
	return nil
}

func (cm *CommandMessage) Do() (err error) {
	var err1, err2, err3, err4 error
	switch cm.CommandName {
	case CONNECT:
		err1 = NewWindowAcknowledgeSizeMessage(cm.MessageBase, uint32(2500000)).Send()
		err2 = NewSetPeerBandWidthMessage(cm.MessageBase, uint32(2500000), 0x02).Send()
		//XXX: can set peer chunk size here
		err3 = NewUserControlMessage(cm.MessageBase, StreamBegin).Send()
		err4 = NewCommandMessageResponse(cm.MessageBase, cm.CommandName, _RESULT, cm.TranscationID, 0).Send()
		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4)
	case RELEASE_STREAM, FCPUBLISH: //ignore
	case CALL:
	case CLOSE:
	case CREATE_STREAM:
		err = NewCommandMessageResponse(cm.MessageBase, cm.CommandName, _RESULT, cm.TranscationID, cm.messageStreamID).Send()
	case PUBLISH:
		err = NewCommandMessageResponse(cm.MessageBase, cm.CommandName, ON_STATUS, cm.TranscationID, 0).Send()
	case PLAY:
		err1 := NewCommandMessageResponse(cm.MessageBase, cm.CommandName, ON_STATUS, cm.TranscationID, 0).Send()
		// err2 := NewDataMessage(cm.MessageBase).Send()
		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
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
	Capabilities   float64 `structs:"capabilities,omitempty"`
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

func NewCommandMessageResponse(mb MessageBase, commandName, commandRespName string, transcationID int, streamID uint32) (cm *CommandMessageResponse) {
	cmr := &CommandMessageResponse{
		MessageBase:     mb,
		CommandName:     commandName,
		CommandRespName: commandRespName,
		TranscationID:   transcationID,
	}

	switch commandName {
	case CONNECT:
		cmr.CommandObject.FmsVer = "FMS/3,0,1,123"
		// cmr.CommandObject.Capabilities=31
		// cmr.CommandObject.Level = "status"
		// cmr.CommandObject.Code = "NetConnection.Connect.Success"
		// cmr.CommandObject.Description = "Connection succeeded"
		// cmr.CommandObject.ObjectEncoding = 0
	case CREATE_STREAM:
		cmr.StreamID = streamID
	case PUBLISH:
		cmr.CommandObject.Level = "status"
		cmr.CommandObject.Code = "NetStream.Publish.Start"
		cmr.CommandObject.Description = "Start publishing"
	case PLAY:
		cmr.CommandObject.Level = "status"
		cmr.CommandObject.Code = "NetStream.Play.Start"
		cmr.CommandObject.Description = "Start play"
	default: //do nothing
	}
	return cmr
}

// func (cmr *CommandMessageResponse) Do() error {
// return nil
// }

func (cmr *CommandMessageResponse) Send() (err error) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := amf_pkg.AMF0

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
	case PLAY:
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
	//TODO: split message in multi chunk
	return NewChunk(COMMAND_MESSAGE_AMF0, FMT0, b).Send(cmr.rtmp)
}
