package rtmp

//https://www.adobe.com/content/dam/acom/en/devnet/flv/video_file_format_spec_v10.pdf

type TAG_TYPE int

const (
	VIDEO_TAG       TAG_TYPE = 8
	AUDIO_TAG       TAG_TYPE = 9
	SCRIPT_DATA_TAG TAG_TYPE = 18
)

type Packet struct {
	Type              TAG_TYPE
	DataSize          uint32 //uint24
	TimeStamp         uint32 //uint24
	TimeStampExtended uint8
	StreamID          uint32 //uint23, always 0
	Data              []byte
}

func NewPacket(message Message) (packet *Packet) {
	mb := message.GetInfo()
	packet = &Packet{
		// Type     :,
		// DataSize    :,
		TimeStamp: mb.messageTime,
		// TimeStampExtended :,
		StreamID: mb.messageStreamID,
		Data:     mb.messagePayload,
	}
	switch mb.messageType {
	case AUDIO_MESSAGE:
		packet.Type = AUDIO_TAG
	case VIDEO_MESSAGE:
		packet.Type = VIDEO_TAG
	case DATA_MESSAGE_AMF0, DATA_MESSAGE_AMF3:
		packet.Type = SCRIPT_DATA_TAG
	default:
		return nil
	}
	return packet
}
