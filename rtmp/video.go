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
	from     string
	index    int
}

func NewVideoMessage(mb MessageBase, from string, index int, fields ...interface{}) (vm *VideoMessage) {
	vm = &VideoMessage{
		MessageBase: mb,
		from:        from,
		index:       index,
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
		format := FMT0
		if i != 0 {
			format = FMT3
		}

		lIndex := i * int(vm.rtmp.ownMaxChunkSize)
		rIndex := (i + 1) * int(vm.rtmp.ownMaxChunkSize)
		if rIndex > len(vm.messagePayload) {
			rIndex = len(vm.messagePayload)
			i = -2
		}
		NewChunk(VIDEO_MESSAGE, uint32(len(vm.messagePayload)), vm.messageTime, format, 9, vm.messagePayload[lIndex:rIndex]).Send(vm.rtmp)
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
	if vm.videoTag.FrameType == flv.KEY_FRAME && vm.videoTag.AVCPacketType == flv.AVC_SEQUENCE_HEADER {
		vm.rtmp.room.VideoSeq = vm.videoTag
		return nil
	}

	if vm.videoTag.FrameType == flv.KEY_FRAME {
		vm.rtmp.room.GOP = vm.rtmp.room.GOP[0:0:cap(vm.rtmp.room.GOP)]
	}

	vm.rtmp.room.GOP = append(vm.rtmp.room.GOP, vm.videoTag)
	vm.rtmp.room.ch <- vm.videoTag
	return nil
}
