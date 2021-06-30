package rtmp

import (
	"github.com/SmartBrave/GGmpeg/flv"
)

type VideoCodec float64

const (
	SUPPORT_VID_UNUSE VideoCodec = 0x0001 << iota
	SUPPORT_VID_JPEG
	SUPPORT_VID_SORENSON
	SUPPORT_VID_HOMEBREW
	SUPPORT_VID_VP6
	SUPPORT_VID_VP6ALPHA
	SUPPORT_VID_HOMEBREWV
	SUPPORT_VID_H264
	SUPPORT_VID_ALL = 0x00ff
)

type VideoFunction float64

const (
	SUPPORT_VID_CLIENT_SEEK VideoFunction = 1
)

type VideoMessage struct {
	MessageBase
	videoTag *flv.VideoTag
}

func NewVideoMessage(mb MessageBase) (vm *VideoMessage) {
	return &VideoMessage{
		MessageBase: mb,
	}
}

func (vm *VideoMessage) Send() (err error) {
	for i := 0; i >= 0; i++ {
		lIndex := i * int(vm.rtmp.peerMaxChunkSize)
		rIndex := (i + 1) * int(vm.rtmp.peerMaxChunkSize)
		if rIndex > len(vm.messagePayload) {
			rIndex = len(vm.messagePayload)
			i = -2
		}
		NewChunk(VIDEO_MESSAGE, FMT0, vm.messagePayload[lIndex:rIndex]).Send(vm.rtmp)
	}
	return nil
}

func (vm *VideoMessage) Parse() (err error) {
	vm.videoTag, err = flv.ParseVideoTag(&flv.TagBase{
		TagType:   VIDEO_MESSAGE,
		DataSize:  vm.messageLength,
		TimeStamp: vm.messageTime,
		StreamID:  0,
	}, vm.messagePayload)
	if err != nil {
		return err
	}

	return nil
}

func (vm *VideoMessage) Do() (err error) {
	vm.rtmp.room.Cache.Append(vm.videoTag)
	return nil
}
