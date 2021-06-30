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

type MetaData struct {
	AudioChannels   string  `mapstructure:"audiochannels"`
	AudioCodecID    int     `mapstructure:"audiocodecid"`
	AudioDataRate   int     `mapstructure:"audiodatarate"`
	AudioSampleRate int     `mapstructure:"audiosamplerate"`
	AudioSampleSize int     `mapstructure:"audiosamplesize"`
	Author          string  `mapstructure:"author"`
	Company         string  `mapstructure:"company"`
	DisplayHeight   string  `mapstructure:"displayheight"`
	DisplayWidth    string  `mapstructure:"displaywidth"`
	Duration        int     `mapstructure:"duration"`
	Encoder         string  `mapstructure:"encoder"`
	FileSize        int     `mapstructure:"filesize"`
	Fps             string  `mapstructure:"fps"`
	FrameRate       int     `mapstructure:"framerate"`
	Height          int     `mapstructure:"height"`
	Level           string  `mapstructure:"level"`
	Profile         string  `mapstructure:"profile"`
	Stereo          bool    `mapstructure:"stereo"`
	Version         string  `mapstructure:"version"`
	VideoCodecID    int     `mapstructure:"videocodecid"`
	VideoDataRate   float64 `mapstructure:"videodatarate"`
	Width           int     `mapstructure:"width"`
}

type DataMessage struct {
	MessageBase
	FirstField  string
	SecondField string
	MetaData    MetaData
}

func NewDataMessage(mb MessageBase) (dm *DataMessage) {
	return &DataMessage{
		MessageBase: mb,
	}
}

func (dm *DataMessage) Send() (err error) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := amf_pkg.AMF0

	err1 := amf.Encode(writer, dm.FirstField)
	err2 := amf.Encode(writer, dm.SecondField)
	err3 := amf.Encode(writer, structs.Map(dm.MetaData))
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

	for i := 0; i >= 0; i++ {
		lIndex := i * int(dm.rtmp.peerMaxChunkSize)
		rIndex := (i + 1) * int(dm.rtmp.peerMaxChunkSize)
		if rIndex > len(b) {
			rIndex = len(b)
			i = -2
		}
		NewChunk(DATA_MESSAGE_AMF0, FMT0, b[lIndex:rIndex]).Send(dm.rtmp)
	}
	return nil
}

func (dm *DataMessage) Parse() (err error) {
	var array []interface{}
	array, err = dm.amf.Decode(easyio.NewEasyReader(bytes.NewReader(dm.messagePayload)))
	if err != nil {
		return err
	}

	for index, a := range array {
		fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	}

	dm.FirstField = array[0].(string)
	dm.SecondField = array[1].(string)
	err = mapstructure.Decode(array[2], &dm.MetaData)
	if err != nil {
		return errors.Wrap(err, "mapstructure.Decode")
	}
	return nil
}

func (dm *DataMessage) Do() (err error) {
	dm.rtmp.room.Meta = dm
	return nil
}
