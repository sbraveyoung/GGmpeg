package rtmp

import (
	"fmt"
	"time"

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

func NewVideoMessage(mb MessageBase, fields ...interface{}) (vm *VideoMessage) {
	vm = &VideoMessage{
		MessageBase: mb,
	}
	if len(fields) == 1 {
		var ok bool
		if vm.videoTag, ok = fields[0].(*flv.VideoTag); !ok {
			//TODO
		} else {
			vm.messagePayload = vm.videoTag.Marshal()
		}
	}
	return vm
}

func (vm *VideoMessage) Send() (err error) {
	for i := 0; i >= 0; i++ {
		fmt := FMT0
		if i != 0 {
			fmt = FMT3
		}

		lIndex := i * int(vm.rtmp.peerMaxChunkSize)
		rIndex := (i + 1) * int(vm.rtmp.peerMaxChunkSize)
		if rIndex > len(vm.messagePayload) {
			rIndex = len(vm.messagePayload)
			i = -2
		}
		NewChunk(VIDEO_MESSAGE, fmt, 9, vm.messagePayload[lIndex:rIndex]).Send(vm.rtmp)
	}
	return nil
}

func (vm *VideoMessage) Parse() (err error) {
	vm.videoTag, err = flv.ParseVideoTag(flv.TagBase{
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
	if vm.videoTag.FrameType == flv.KEYFRAME && vm.videoTag.AVCPacketType == flv.AVC_SEQUENCE_HEADER {
		vm.rtmp.room.VideoSeq = vm.videoTag
		fmt.Println(time.Now().Unix(), "1111111111111111111111111111111111111111111111")
		return nil
	}

	if vm.videoTag.FrameType == flv.KEYFRAME {
		vm.rtmp.room.GOP = vm.rtmp.room.GOP[0:0:cap(vm.rtmp.room.GOP)]
		fmt.Println(time.Now().Unix(), "11111111111111111111111111111111111111111111112222222222222222222222222222222222222222")
	}

	vm.rtmp.room.GOP = append(vm.rtmp.room.GOP, vm.videoTag)
	vm.rtmp.room.ch <- 1
	return nil
}
