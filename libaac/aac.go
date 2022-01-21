package libaac

import (
	"fmt"
)

var (
	AACRates = [...]int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}
)

type AACHeader struct {
	ObjectType uint8
	SampleRate uint8
	Channel    uint8
}

//XXX: copyed from livego
func (ah *AACHeader) Parse(data []byte) (err error) {
	if len(data) < 2 {
		return fmt.Errorf("invalid data, length:%d", len(data))
	}

	ah.ObjectType = (data[0] >> 3) & 0xff
	ah.SampleRate = ((data[0] & 0x07) << 1) | data[1]>>7
	ah.Channel = (data[1] >> 3) & 0x0f
	return nil
}

func (ah *AACHeader) Adts(data []byte) (header []byte) {
	frameLen := uint16(len(data)) + 7
	return []byte{
		0xff,
		0xf1,
		((ah.ObjectType - 1) << 6) | (ah.SampleRate << 2),
		((ah.Channel << 2) << 4) | uint8((frameLen<<3)>>14),
		uint8((frameLen << 5) >> 8),
		uint8(((frameLen<<13)>>13)<<5) | ((0x7c << 1) >> 3),
		0xfc,
	}
}
