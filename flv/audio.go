package flv

import "errors"

const (
	LINER_PCM_PLATFORM_ENDIAN   uint8 = 0
	ADPCM                             = 1
	MP3                               = 2
	LINER_PCM_LITTLE_ENDIAN           = 3
	NELLYMOSER_16KHZ_MONO             = 4
	NELLYMOSER_8KHZ_MONO              = 5
	NELLYMOSER                        = 6
	G711_A_LOW_LOGARITHMIC_PCM        = 7
	G711_MU_LOW_LOGARITHMIC_PCM       = 8
	AAC                               = 10
	SPEEX                             = 11
	MP3_8KHZ                          = 14
	DEVICE_SPECIFIC_SOUND             = 15
)

const (
	_5_5KHZ uint8 = 0
	_11KHZ        = 1
	_22KHZ        = 2
	_44HKZ        = 3 //for AAC, always 3
)

const (
	SND_8BIT   uint8 = 0
	SND_16_BIT       = 1
)

const (
	SND_MONO   uint8 = 0 //for Nellymoser, always 0
	SND_STEREO       = 1 //for AAC, always 1
)

type AudioTag struct {
	TagBase
	SoundFormat   uint8 //4bits
	SoundRate     uint8 //2bits
	SoundSize     uint8 //1bit
	SoundType     uint8 //1bit
	SoundData     []byte
	AACPacketType uint8
}

func ParseAudioTag(tb TagBase, b []byte) (audio *AudioTag, err error) {
	if len(b) <= 1 {
		return nil, errors.New("invalid audio format")
	}
	audio = &AudioTag{
		TagBase:     tb,
		SoundFormat: b[0] >> 4,
		SoundRate:   (b[0] >> 2) & 0x03,
		SoundSize:   (b[0] >> 1) & 0x01,
		SoundType:   b[0] & 0x01,
		SoundData:   b[1:],
	}

	if audio.SoundFormat == AAC {
		if len(b) <= 2 {
			return nil, errors.New("invalid audio format")
		}
		audio.AACPacketType = b[1]
		audio.SoundData = audio.SoundData[1:]
	}

	return audio, nil
}

func (at *AudioTag) Marshal() (b []byte) {
	b = make([]byte, 0, 1)

	b = append(b, (at.SoundFormat<<4)|((at.SoundRate&0x03)<<2)|((at.SoundSize&0x01)<<1)|(at.SoundType&0x01))
	if at.SoundFormat == AAC {
		b = append(b, at.AACPacketType)
		b = append(b, at.SoundData...)
	}
	return b
}
