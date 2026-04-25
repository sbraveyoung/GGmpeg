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

	meta = &MetaTag{TagBase: tb}
	//Publishers emit data messages in two shapes:
	//  - [string "onMetaData", ecma-array]
	//  - [string "@setDataFrame", string "onMetaData", ecma-array]
	//Canonicalise to always keep the onMetaData string in SecondField
	//and the properties array in the last slot.
	var props interface{}
	switch {
	case len(array) >= 3:
		if s, ok := array[0].(string); ok {
			meta.FirstField = s
		}
		if s, ok := array[1].(string); ok {
			meta.SecondField = s
		}
		props = array[2]
	case len(array) == 2:
		if s, ok := array[0].(string); ok {
			meta.SecondField = s
		}
		props = array[1]
	default:
		return meta, errors.New("invalid onMetaData payload")
	}

	if props != nil {
		if err = mapstructure.Decode(props, meta); err != nil {
			fmt.Printf("decode data error, err:%+v\n", err)
			err = errors.Wrap(err, "mapstructure.Decode data")
		}
	}
	return meta, err
}

// Marshal serialises the metadata tag body in the form FLV players
// expect: two AMF0 elements — the string "onMetaData" followed by the
// ECMA array of properties. (RTMP's @setDataFrame wrapper is a command
// verb, not part of the FLV script-data tag format, so we drop
// FirstField even when Parse observed one.)
func (mt *MetaTag) Marshal() (b []byte) {
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := libamf.AMF0

	name := mt.SecondField
	if name == "" {
		name = "onMetaData"
	}

	var err1, err2 error
	err1 = amf.Encode(writer, name)
	err2 = amf.Encode(writer, structs.Map(mt))
	if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
		fmt.Println("HandleMultiError error:", err)
		return nil
	}

	var err error
	b, err = io.ReadAll(buf)
	if err != nil {
		return nil
	}
	return b
}

func (mt *MetaTag) Data() (b []byte) {
	return mt.Marshal()
}
