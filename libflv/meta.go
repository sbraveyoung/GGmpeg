package libflv

import (
	"bytes"

	"github.com/SmartBrave/GGmpeg/libamf"
	"github.com/SmartBrave/utils_sb/easyio"
	"github.com/goinggo/mapstructure"
	"github.com/pkg/errors"
)

type MetaTag struct {
	TagBase
	FirstField      string
	SecondField     string
	AudioChannels   float64 `mapstructure:"audiochannels"`
	AudioCodecID    string  `mapstructure:"audiocodecid"`
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
	VideoCodecID    string  `mapstructure:"videocodecid"`
	VideoDataRate   float64 `mapstructure:"videodatarate"`
	Width           int     `mapstructure:"width"`
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

	err = mapstructure.Decode(array[2], &meta)
	if err != nil {
		err = errors.Wrap(err, "mapstructure.Decode")
	}

	return meta, err
}

func (mt *MetaTag) Marshal() (b []byte) {
	return b
}
