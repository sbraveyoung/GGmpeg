package libflv

import (
	"bytes"
	"fmt"
	"io"

	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libamf"
	"github.com/fatih/structs"
	"github.com/goinggo/mapstructure"
	"github.com/pkg/errors"
)

type MetaTag struct {
	TagBase         `structs:"-"`
	FirstField      string  `structs:"-"`
	SecondField     string  `structs:"-"`
	AudioChannels   string  `mapstructure:"audiochannels" structs:"audiochannels"`
	AudioCodecID    float64 `mapstructure:"audiocodecid" structs:"audiocodecid"`
	AudioDataRate   int     `mapstructure:"audiodatarate" structs:"audiodatarate"`
	AudioSampleRate int     `mapstructure:"audiosamplerate" structs:"audiosamplerate"`
	AudioSampleSize int     `mapstructure:"audiosamplesize" structs:"audiosamplesize"`
	Author          string  `mapstructure:"author" structs:"author"`
	Company         string  `mapstructure:"company" structs:"company"`
	DisplayHeight   string  `mapstructure:"displayHeight" structs:"displayHeight"`
	DisplayWidth    string  `mapstructure:"displayWidth" structs:"displayWidth"`
	Duration        int     `mapstructure:"duration" structs:"duration"`
	Encoder         string  `mapstructure:"encoder" structs:"encoder"`
	FileSize        int     `mapstructure:"filesize" structs:"filesize"`
	Fps             string  `mapstructure:"fps" structs:"fps"`
	FrameRate       int     `mapstructure:"framerate" structs:"framerate"`
	Height          int     `mapstructure:"height" structs:"height"`
	Level           string  `mapstructure:"level" structs:"level"`
	Profile         string  `mapstructure:"profile" structs:"profile"`
	Stereo          bool    `mapstructure:"stereo" structs:"stereo"`
	Version         string  `mapstructure:"version" structs:"version"`
	VideoCodecID    float64 `mapstructure:"videocodecid" structs:"videocodecid"`
	VideoDataRate   float64 `mapstructure:"videodatarate" structs:"videodatarate"`
	Width           int     `mapstructure:"width" structs:"width"`
}

func ParseMetaTag(tb TagBase, amf libamf.AMF, b []byte) (meta *MetaTag, err error) {
	var array []interface{}
	array, err = amf.Decode(easyio.NewEasyReader(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}

	//for index, a := range array {
	//	fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	//}

	meta = &MetaTag{
		TagBase:     tb,
		FirstField:  array[0].(string),
		SecondField: array[1].(string),
	}

	err = mapstructure.Decode(array[2], meta)
	if err != nil {
		fmt.Printf("decode data error, err:%+v\n", err)
		err = errors.Wrap(err, "mapstructure.Decode data")
	}

	return meta, err
}

func (mt *MetaTag) Marshal() (b []byte) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := libamf.AMF0

	var err1, err2, err3 error
	//err1 = amf.Encode(writer, mt.FirstField)
	err2 = amf.Encode(writer, mt.SecondField)
	err3 = amf.Encode(writer, structs.Map(mt))
	err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
	if err != nil {
		fmt.Println("HandleMultiError error:", err)
		return nil
	}

	b, err = io.ReadAll(buf)
	if err != nil {
		return nil
	}
	return b
}

func (mt *MetaTag) Data() (b []byte) {
	//XXX
	return
}
