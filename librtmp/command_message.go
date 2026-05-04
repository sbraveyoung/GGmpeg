package librtmp

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sync/atomic"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/sbraveyoung/GGmpeg/libamf"
	"github.com/sbraveyoung/GGmpeg/libdash"
	"github.com/sbraveyoung/GGmpeg/libhls"
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

	// Pause / Seek fields.
	PauseFlag    bool    //pause
	MilliSeconds float64 //pause, seek

	// Reader flag toggles for receiveAudio / receiveVideo.
	BoolFlag bool
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
	}
	return cm
}

// Send is only used when this server originates a command — which, given
// we don't act as an RTMP client, never happens. Responses flow through
// CommandMessageResponse instead.
func (cm *CommandMessage) Send() (err error) {
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
	case RELEASE_STREAM, FCPUBLISH, FCUNPUBLISH:
		if len(array) >= 4 {
			cm.PublishingName, _ = array[3].(string)
		}
	case DELETE_STREAM, CLOSE_STREAM:
		if len(array) >= 4 {
			if f, ok := array[3].(float64); ok {
				cm.MessageBase.messageStreamID = uint32(f)
			}
		}
	case CREATE_STREAM: //do nothing
	case PUBLISH:
		if len(array) < 5 {
			return errors.New("invalid publish command")
		}
		cm.PublishingName, _ = array[3].(string)
		cm.PublishingType, _ = array[4].(string)
	case PLAY:
		if len(array) < 4 {
			return errors.New("invalid play command")
		}
		cm.PublishingName, _ = array[3].(string)
	case PAUSE:
		//pause(cmd, txn, null, pauseFlag, milliSeconds)
		if len(array) >= 5 {
			cm.PauseFlag, _ = array[3].(bool)
			cm.MilliSeconds, _ = array[4].(float64)
		}
	case SEEK:
		if len(array) >= 4 {
			cm.MilliSeconds, _ = array[3].(float64)
		}
	case RECEIVE_AUDIO, RECEIVE_VIDEO:
		if len(array) >= 4 {
			cm.BoolFlag, _ = array[3].(bool)
		}
	}
	return nil
}

func (cm *CommandMessage) Do() (err error) {

	var err1, err2, err3, err4, err5 error
	switch cm.CommandName {
	case CONNECT:
		if _, ok := cm.rtmp.server.apps[cm.CommandObject.App]; !ok {
			return errors.Errorf("unknown app: %s", cm.CommandObject.App)
		}
		cm.rtmp.app = cm.CommandObject.App
		err1 = NewWindowAcknowledgeSizeMessage(cm.MessageBase, uint32(2500000)).Send()
		cm.rtmp.ownWindowAckSize = 2500000
		err2 = NewSetPeerBandWidthMessage(cm.MessageBase, uint32(2500000), DYNAMIC).Send()
		//BUG: If do not set ownChunkSize or set ownChunkSize to other value, player maybe panic. But I don't know the reason.
		cm.rtmp.ownMaxChunkSize = 4096
		err3 = NewSetChunkSizeMessage(cm.MessageBase, uint32(cm.rtmp.ownMaxChunkSize)).Send()
		err4 = NewUserControlMessage(cm.MessageBase, StreamBegin).Send()
		err5 = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: _RESULT,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				FmsVer:         "FMS/3,0,1,123",
				Capabilities:   31,
				Level:          "status",
				Code:           "NetConnection.Connect.Success",
				Description:    "Connection succeeded",
				ObjectEncoding: cm.CommandObject.ObjectEncoding,
			},
		}).Send()
		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4, err5)

	case RELEASE_STREAM, FCPUBLISH:
		//Both commands are client-side hints emitted by FMLE/OBS before
		//publish. Reply with a minimal _result so the publisher's state
		//machine moves forward instead of retrying.
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: _RESULT,
			TranscationID:   cm.TranscationID,
			NullValue:       true,
		}).Send()

	case FCUNPUBLISH:
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Unpublish.Success",
				Description: "Stop publishing",
			},
		}).Send()

	case CALL:
	case CLOSE:
	case CREATE_STREAM:
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: _RESULT,
			TranscationID:   cm.TranscationID,
			StreamID:        cm.messageStreamID,
		}).Send()

	case PUBLISH:
		app, ok := cm.rtmp.server.apps[cm.rtmp.app]
		if !ok {
			return errors.Errorf("app not found: %s", cm.rtmp.app)
		}
		if existing := app.Load(cm.PublishingName); existing != nil {
			//Refuse a second publish to the same stream — replacing the
			//publisher mid-stream would desync every viewer.
			return (&CommandMessageResponse{
				MessageBase:     cm.MessageBase,
				CommandName:     cm.CommandName,
				CommandRespName: ON_STATUS,
				TranscationID:   cm.TranscationID,
				CommandObject: ConnectRespCommandObject{
					Level:       "error",
					Code:        "NetStream.Publish.BadName",
					Description: "Stream already publishing",
				},
			}).Send()
		}
		cm.rtmp.room = NewRoom(cm.rtmp, cm.PublishingName)
		cm.rtmp.role = rolePublisher
		app.Store(cm.PublishingName, cm.rtmp.room)

		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Publish.Start",
				Description: "Start publishing",
			},
		}).Send()

		if app.hlsMode == libhls.IMMEDIATELY {
			hls := libhls.NewHls().WithStreamID(cm.PublishingName).WithDir(app.hlsDir)
			app.hls.Store(cm.PublishingName, hls)
			go hls.Start(broadcast.NewBroadcastReader(cm.rtmp.room.GOP))
		}
		if app.dashEnabled {
			dash := libdash.NewDASH().WithStreamID(cm.PublishingName).WithDir(app.dashDir)
			app.StoreDASH(cm.PublishingName, dash)
			go dash.Start(broadcast.NewBroadcastReader(cm.rtmp.room.GOP))
		}

	case PLAY:
		app, ok := cm.rtmp.server.apps[cm.rtmp.app]
		if !ok {
			return errors.Errorf("app not found: %s", cm.rtmp.app)
		}
		cm.rtmp.room = app.Load(cm.PublishingName)
		if cm.rtmp.room == nil {
			return (&CommandMessageResponse{
				MessageBase:     cm.MessageBase,
				CommandName:     cm.CommandName,
				CommandRespName: ON_STATUS,
				TranscationID:   cm.TranscationID,
				CommandObject: ConnectRespCommandObject{
					Level:       "error",
					Code:        "NetStream.Play.StreamNotFound",
					Description: "Stream not found",
				},
			}).Send()
		}
		cm.rtmp.role = rolePlayer
		cm.rtmp.room.RTMPJoin(cm.rtmp)

		err1 = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Play.Reset",
				Description: "Start play",
			},
		}).Send()
		err2 = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Play.Start",
				Description: "Start play",
			},
		}).Send()
		err3 = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Data.Start",
				Description: "Start play",
			},
		}).Send()
		err4 = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Play.PublishNotify",
				Description: "Start play notify",
			},
		}).Send()
		err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4, err5)

	case PAUSE:
		code := "NetStream.Unpause.Notify"
		if cm.PauseFlag {
			code = "NetStream.Pause.Notify"
		}
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        code,
				Description: "Pause/Unpause notification",
			},
		}).Send()

	case SEEK:
		//We don't support true seeks on a live broadcast. Acknowledge so
		//the client doesn't hang waiting for a response.
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Seek.Notify",
				Description: "Seeking",
			},
		}).Send()

	case RECEIVE_AUDIO, RECEIVE_VIDEO:
		//No-op ack; we don't implement per-track toggling for live GOP
		//replay.
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: _RESULT,
			TranscationID:   cm.TranscationID,
			NullValue:       true,
		}).Send()

	case DELETE_STREAM, CLOSE_STREAM:
		//Stream close emitted by OBS when the user stops streaming.
		//We rely on cleanup() in HandlerServer for the actual teardown;
		//here we just reply with a status update so the client can
		//proceed to disconnect.
		err = (&CommandMessageResponse{
			MessageBase:     cm.MessageBase,
			CommandName:     cm.CommandName,
			CommandRespName: ON_STATUS,
			TranscationID:   cm.TranscationID,
			CommandObject: ConnectRespCommandObject{
				Level:       "status",
				Code:        "NetStream.Unpublish.Success",
				Description: "Stream closed",
			},
		}).Send()

	case _RESULT:
		//Outbound-client path: bump the response counter so the
		//connect/createStream sender can move on. No-op for the more
		//common server-side case.
		atomic.AddUint32(&cm.rtmp.resultCount, 1)
	case _ERROR:
		atomic.AddUint32(&cm.rtmp.resultCount, 1)
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
	// When NullValue is true, encode a null property object instead of
	// the command-object struct — used for simple _result replies to
	// releaseStream / FCPublish.
	NullValue bool
}

func (cmr *CommandMessageResponse) Send() (err error) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := libamf.AMF0

	var err1, err2, err3, err4 error
	err1 = amf.Encode(writer, cmr.CommandRespName)
	err2 = amf.Encode(writer, cmr.TranscationID)
	switch {
	case cmr.NullValue:
		err3 = amf.Encode(writer, nil)
		err4 = amf.Encode(writer, nil)
	case cmr.CommandName == CONNECT:
		err3 = amf.Encode(writer, structs.Map(cmr.CommandObject))
	case cmr.CommandName == CREATE_STREAM:
		err3 = amf.Encode(writer, nil)
		err4 = amf.Encode(writer, cmr.StreamID)
	case cmr.CommandName == PUBLISH, cmr.CommandName == PLAY,
		cmr.CommandName == PAUSE, cmr.CommandName == SEEK,
		cmr.CommandName == DELETE_STREAM, cmr.CommandName == CLOSE_STREAM,
		cmr.CommandName == FCUNPUBLISH:
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
	chunkSize := cmr.rtmp.ownMaxChunkSize
	for i := 0; ; i++ {
		lIndex := i * chunkSize
		if lIndex >= len(b) {
			break
		}
		rIndex := lIndex + chunkSize
		if rIndex > len(b) {
			rIndex = len(b)
		}
		fmtType := FMT0
		if i != 0 {
			fmtType = FMT3
		}
		if sendErr := NewChunk(COMMAND_MESSAGE_AMF0, uint32(len(b)), cmr.messageTime, fmtType, csidCommand, b[lIndex:rIndex]).Send(cmr.rtmp); sendErr != nil {
			return sendErr
		}
	}
	return nil
}
