package rtmp

import (
	"bytes"
	"fmt"

	"github.com/SmartBrave/GGmpeg/flv"
	"github.com/SmartBrave/utils/easyio"
)

type AudioCodec float64

const (
	SUPPORT_SND_NONE AudioCodec = 0x0001 << iota
	SUPPORT_SND_ADPCM
	SUPPORT_SND_MP3
	SUPPORT_SND_INTEL
	SUPPORT_SND_UNUSED
	SUPPORT_SND_NELLY8
	SUPPORT_SND_NELLY
	SUPPORT_SND_G711A
	SUPPORT_SND_G711U
	SUPPORT_SND_NELLY16
	SUPPORT_SND_AAC
	SUPPORT_SND_SPEEX
	SUPPORT_SND_ALL = 0x0fff
)

type AudioMessage struct {
	MessageBase
	tag *flv.Tag
}

func NewAudioMessage(mb MessageBase) (am *AudioMessage) {
	return &AudioMessage{
		MessageBase: mb,
	}
}

func (am *AudioMessage) Send() (err error) {
	//TODO
	fmt.Println("-------------------------------------------------send audio--------------------------------------")
	return nil
}

func (am *AudioMessage) Parse() (err error) {
	fmt.Printf("audio message length:%d\npayload:%x\n", len(am.messagePayload), am.messagePayload)
	am.tag, err = flv.ParseTag(easyio.NewEasyReader(bytes.NewReader(am.messagePayload)))
	if err != nil {
		return err
	}

	return nil
}

func (am *AudioMessage) Do() (err error) {
	am.rtmp.room.Cache.Append(am.tag)
	return nil
}
