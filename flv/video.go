package flv

import "errors"

const ( //frame_type
	KEYFRAME                 uint8 = 1 //for AVC, a seekable frame
	INTER_FRAME                    = 2 //for AVC, a non-seekable frame
	DISPOSABLE_INTER_FRAME         = 3 //H.263 only
	GENERATED_KEYFRAME             = 4 //reserved for server use only
	VIDEO_INFO_COMMAND_FRAME       = 5
)

const ( //codec_id
	JPEG                       uint8 = 1 //currently unused
	SORENSON_H263                    = 2
	SCREEN_VIDEO                     = 3
	ON2_VP6                          = 4
	ON2_VP6_WITH_ALPHA_CHANNEL       = 5
	SCREEN_VIDEO_VERSION2            = 6
	AVC                              = 7
)

const ( //avc_packet_type
	AVC_SEQUENCE_HEADER = 1
	AVC_NALU            = 2
	AVC_END_OF_SEQUENCE = 3
)

type Video struct {
	FrameType       uint8 //4bits
	CodecID         uint8 //4bits
	VideoData       []byte
	AVCPacketType   uint8
	CompositionTime uint32 //int24
}

func ParseVideo(b []byte) (video *Video, err error) {
	if len(b) < 1 {
		return nil, errors.New("invalid video format")
	}

	video = &Video{
		FrameType: (b[0] >> 4) & 0x0f,
		CodecID:   b[0] & 0x0f,
		VideoData: b[1:],
	}

	switch video.CodecID {
	case JPEG, SORENSON_H263, SCREEN_VIDEO, ON2_VP6, ON2_VP6_WITH_ALPHA_CHANNEL, SCREEN_VIDEO_VERSION2:
		//ignore
	case AVC:
		video.AVCPacketType = b[1]
		if video.AVCPacketType == AVC_NALU {
			video.CompositionTime = uint32(0x00)<<24 | uint32(b[2])<<16 | uint32(b[3])<<8 | uint32(b[4])
		}
		video.VideoData = video.VideoData[5:]
	}

	return video, nil
}
