package librtmp

import (
	"fmt"
	"time"

	"github.com/SmartBrave/GGmpeg/libflv"
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
	audioTag *libflv.AudioTag
}

func NewAudioMessage(mb MessageBase, fields ...interface{}) (am *AudioMessage) {
	am = &AudioMessage{
		MessageBase: mb,
	}
	if len(fields) == 1 {
		var ok bool
		if am.audioTag, ok = fields[0].(*libflv.AudioTag); !ok {
			//TODO
		} else {
			am.messagePayload = am.audioTag.Marshal()
		}
	}
	return am
}

func (am *AudioMessage) Send() (err error) {
	chunkSize := am.rtmp.ownMaxChunkSize
	for i := 0; ; i++ {
		lIndex := i * chunkSize
		if lIndex >= len(am.messagePayload) {
			break
		}
		rIndex := lIndex + chunkSize
		if rIndex > len(am.messagePayload) {
			rIndex = len(am.messagePayload)
		}
		fmtType := FMT0
		if i != 0 {
			fmtType = FMT3
		}
		if sendErr := NewChunk(AUDIO_MESSAGE, uint32(len(am.messagePayload)), am.messageTime, fmtType, csidAudio, am.messagePayload[lIndex:rIndex]).Send(am.rtmp); sendErr != nil {
			return sendErr
		}
	}
	return nil
}

func (am *AudioMessage) Parse() (err error) {
	am.audioTag, err = libflv.ParseAudioTag(libflv.TagBase{
		TagType:   libflv.AUDIO_TAG,
		TimeStamp: am.messageTime,
		StreamID:  0,
	}, am.messagePayload)
	if err != nil {
		return err
	}
	am.audioTag.DataSize = uint32(len(am.audioTag.Data()))
	return nil
}

func (am *AudioMessage) Do() (err error) {
	if am.rtmp.room == nil {
		return nil
	}
	if am.audioTag.SoundFormat == libflv.FLV_AUDIO_AAC && am.audioTag.AACPacketType == libflv.AAC_SEQUENCE_HEADER {
		am.rtmp.room.setAudioSequenceHeader(am.audioTag)
		am.rtmp.room.GOP.WriteMeta(am.audioTag)
		fmt.Printf("write packet audio :%+v\n", am.audioTag)
		return nil
	}
	fmt.Printf("[gop receive audio] message time(dts):%d, now:%+v\n", am.messageTime, time.Now())
	am.rtmp.room.GOP.Write(am.audioTag)
	return nil
}
