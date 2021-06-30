package rtmp

import (
	"github.com/SmartBrave/GGmpeg/flv"
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
	tag flv.Tag
}

func NewAudioMessage(mb MessageBase) (am *AudioMessage) {
	return &AudioMessage{
		MessageBase: mb,
	}
}

func (am *AudioMessage) Send() (err error) {
	for i := 0; i >= 0; i++ {
		lIndex := i * int(am.rtmp.peerMaxChunkSize)
		rIndex := (i + 1) * int(am.rtmp.peerMaxChunkSize)
		if rIndex > len(am.messagePayload) {
			rIndex = len(am.messagePayload)
			i = -2
		}
		NewChunk(VIDEO_MESSAGE, FMT0, am.messagePayload[lIndex:rIndex]).Send(am.rtmp)
	}
	return nil
}

func (am *AudioMessage) Parse() (err error) {
	am.tag, err = flv.ParseAudioTag(flv.TagBase{
		TagType:   AUDIO_MESSAGE,
		DataSize:  am.messageLength,
		TimeStamp: am.messageTime,
		StreamID:  0,
	}, am.messagePayload)
	if err != nil {
		return err
	}

	return nil
}

func (am *AudioMessage) Do() (err error) {
	am.rtmp.room.Cache.Append(am.tag)
	return nil
}
