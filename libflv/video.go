package libflv

import (
	"errors"
	"fmt"
	"time"
)

const ( //frame_type
	KEY_FRAME                uint8 = 1 //for AVC, a seekable frame
	INTER_FRAME                    = 2 //for AVC, a non-seekable frame
	DISPOSABLE_INTER_FRAME         = 3 //H.263 only
	GENERATED_KEYFRAME             = 4 //reserved for server use only
	VIDEO_INFO_COMMAND_FRAME       = 5
)

const ( //codec_id
	// JPEG                       uint8 = 1
	FLV_VIDEO_SORENSON_H263 = 2
	// FLV_VIDEO_SCREEN_VIDEO               = 3
	FLV_VIDEO_VP6 = 4 //ON2_VP6
	// FLV_VIDEO_ON2_VP6_WITH_ALPHA_CHANNEL = 5
	// FLV_VIDEO_SCREEN_VIDEO_VERSION2      = 6
	FLV_VIDEO_AVC  = 7
	FLV_VIDEO_HEVC = 12 //https://github.com/CDN-Union/H265
	FLV_VIDEO_AV1  = 13 //https://aomediacodec.github.io/av1-isobmff
)

const ( //avc_packet_type
	AVC_SEQUENCE_HEADER = 0
	AVC_NALU            = 1
	AVC_END_OF_SEQUENCE = 2
)

type VideoTag struct {
	TagBase
	FrameType     uint8 //4bits
	CodecID       uint8 //4bits
	VideoData     []byte
	AVCPacketType uint8
	Cts           uint32 //CompositionTime, int24
}

func ParseVideoTag(tb TagBase, b []byte) (video *VideoTag, err error) {
	if len(b) < 1 {
		return nil, errors.New("invalid video format")
	}

	video = &VideoTag{
		TagBase:   tb,
		FrameType: (b[0] >> 4) & 0x0f,
		CodecID:   b[0] & 0x0f,
		VideoData: b[1:],
		// VideoData: b,
	}

	if video.CodecID == FLV_VIDEO_AVC || video.CodecID == FLV_VIDEO_HEVC || video.CodecID == FLV_VIDEO_AV1 {
		if len(b) < 5 {
			return nil, errors.New("invalid video format")
		}
		video.AVCPacketType = b[1]
		video.Cts = uint32(0x00)<<24 | uint32(b[2])<<16 | uint32(b[3])<<8 | uint32(b[4])
		video.VideoData = b[5:]
	}

	return video, nil
}

func (vt *VideoTag) Marshal() (b []byte) {
	b = append(b, (vt.FrameType<<4)|(vt.CodecID&0x0f))
	switch vt.CodecID {
	case FLV_VIDEO_AVC:
		b = append(b, vt.AVCPacketType)
		b = append(b, uint8((vt.Cts>>16)&0xff), uint8((vt.Cts>>8)&0xff), uint8(vt.Cts&0xff))
	default:
		fmt.Println("vt.CodecID:", vt.CodecID)
		time.Sleep(time.Second)
		panic("video marshal panic")
	}
	b = append(b, vt.VideoData...)
	return b
}

func (vt *VideoTag) Data() (b []byte) {
	return vt.VideoData
}
