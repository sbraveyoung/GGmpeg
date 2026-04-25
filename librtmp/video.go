package librtmp

import (
	"fmt"

	"github.com/SmartBrave/GGmpeg/libflv"
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
	videoTag *libflv.VideoTag
}

func NewVideoMessage(mb MessageBase, fields ...interface{}) (vm *VideoMessage) {
	vm = &VideoMessage{
		MessageBase: mb,
	}
	if len(fields) == 1 {
		var ok bool
		if vm.videoTag, ok = fields[0].(*libflv.VideoTag); !ok {
			//TODO
		} else {
			vm.messagePayload = vm.videoTag.Marshal()
		}
	}
	return vm
}

func (vm *VideoMessage) Send() (err error) {
	chunkSize := vm.rtmp.ownMaxChunkSize
	for i := 0; ; i++ {
		lIndex := i * chunkSize
		if lIndex >= len(vm.messagePayload) {
			break
		}
		rIndex := lIndex + chunkSize
		if rIndex > len(vm.messagePayload) {
			rIndex = len(vm.messagePayload)
		}
		fmtType := FMT0
		if i != 0 {
			fmtType = FMT3
		}
		if sendErr := NewChunk(VIDEO_MESSAGE, uint32(len(vm.messagePayload)), vm.messageTime, fmtType, csidVideo, vm.messagePayload[lIndex:rIndex]).Send(vm.rtmp); sendErr != nil {
			return sendErr
		}
	}
	return nil
}

func (vm *VideoMessage) Parse() (err error) {
	vm.videoTag, err = libflv.ParseVideoTag(libflv.TagBase{
		TagType:   libflv.VIDEO_TAG,
		TimeStamp: vm.messageTime,
		StreamID:  0,
	}, vm.messagePayload)
	if err != nil {
		return err
	}
	vm.videoTag.DataSize = uint32(len(vm.videoTag.Data()))
	return nil
}

func (vm *VideoMessage) Do() (err error) {
	if vm.rtmp.room == nil {
		return nil
	}
	if vm.videoTag.FrameType == libflv.KEY_FRAME {
		if vm.videoTag.AVCPacketType == libflv.AVC_SEQUENCE_HEADER {
			vm.rtmp.room.setVideoSequenceHeader(vm.videoTag)
			vm.rtmp.room.GOP.WriteMeta(vm.videoTag)
		} else {
			vm.rtmp.room.GOP.Reset()
			vm.rtmp.room.GOP.Write(vm.videoTag)
		}
		fmt.Printf("write packet video :%+v\n", vm.videoTag)
	} else {
		vm.rtmp.room.GOP.Write(vm.videoTag)
	}
	return nil
}
