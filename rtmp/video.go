package rtmp

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

func (vm *VideoMessage) Done() bool {
	return true
}

func NewVideoMessage(mb MessageBase) (am *VideoMessage) {
	return &VideoMessage{
		MessageBase: mb,
	}
}

func (am *VideoMessage) Send() (err error) {
	//TODO
	return nil
}

func (am *VideoMessage) Parse() (err error) {
	return nil
}

func (am *VideoMessage) Do() (err error) {
	return nil
}
