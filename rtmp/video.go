package rtmp

import (
	"bytes"

	"github.com/SmartBrave/GGmpeg/flv"
	"github.com/SmartBrave/utils/easyio"
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
}

func NewVideoMessage(mb MessageBase) (vm *VideoMessage) {
	return &VideoMessage{
		MessageBase: mb,
	}
}

func (vm *VideoMessage) Send() (err error) {
	//TODO
	return nil
}

func (vm *VideoMessage) Parse() (err error) {
	tag, err := flv.ParseTag(easyio.NewEasyReader(bytes.NewReader(vm.messagePayload)))
	if err != nil {
		return err
	}

	vm.rtmp.room.Cache.Append(tag)
	return nil
}

func (vm *VideoMessage) Do() (err error) {
	return nil
}
