package rtmp

import (
	"github.com/pkg/errors"
)

type MessageBase struct {
	rtmp                *RTMP
	messageTime         uint32
	messageTimeDelta    uint32
	messageLength       uint32
	messageLengthRemain uint32
	messageType         MessageType
	messageStreamID     uint32
}

func (mb *MessageBase) GetInfo() *MessageBase {
	return mb
}

func (mb *MessageBase) SetInfo(mbNew *MessageBase) {
	mb.rtmp = mbNew.rtmp
	mb.messageTime = mbNew.messageTime
	mb.messageTimeDelta = mbNew.messageTimeDelta
	mb.messageLength = mbNew.messageLength
	mb.messageLengthRemain = mbNew.messageLengthRemain
	mb.messageType = mbNew.messageType
	mb.messageStreamID = mbNew.messageStreamID
}

func (mb *MessageBase) Update(mbNew *MessageBase) {
	//do not update other fields
	mb.messageTimeDelta = mbNew.messageTimeDelta
	mb.messageLengthRemain = mbNew.messageLengthRemain
}

type Message interface {
	Update(*Chunk) error
	Do() error
	GetInfo() *MessageBase
	SetInfo(*MessageBase)
}

// type MessageHeader struct {
// MessageType     uint8
// PayloadLength   uint32 //3bytes
// TimeStamp       uint32
// MessageStreamID uint32 //3bytes
// }

/*
 * MessageType 1-7
   --- control message ---
     -- MessageStreamID: 0
     -- ChunkStreamID: 2
	 -- TimeStamp: ignore
   * 1: set chunk size
   * 2: abort message
   * 3: acknowledgement
   * 4: user control message
   * 5: window acknowledgement size
   * 6: set peer bandwidth
   * 7: TODO is used between edge server and origin server
   --- common message ---
   * 8: audio message
   * 9: video message
   --- command message ---
   * 15(AMF3)/18(AMF0): data message
   * 16(AMF3)/19(AMF0): share object message
   * 17(AMF3)/20(AMF0): command message
   * 22: aggregate message
*/
type MessageType uint8

const (
	//control message
	SET_CHUNK_SIZE              MessageType = iota + 1 //1
	ABORT_MESSAGE                                      //2
	ACKNOWLEDGEMENT                                    //3
	USER_CONTROL_MESSAGE                               //4
	WINDOW_ACKNOWLEDGEMENT_SIZE                        //5
	SET_PEER_BANDWIDTH                                 //6

	//common message
	AUDIO_MESSAGE = iota + 2 //8
	VIDEO_MESSAGE            //9

	//command message
	DATA_MESSAGE_AMF3         = iota + 7 //15
	SHARE_OBJECT_MESSAGE_AMF3            //16
	COMMAND_MESSAGE_AMF3                 //17
	DATA_MESSAGE_AMF0                    //18
	SHARE_OBJECT_MESSAGE_AMF0            //19
	COMMAND_MESSAGE_AMF0                 //20
)

func ParseMessage(rtmp *RTMP, chunk *Chunk) (err error) {
	var message Message
	if chunk.Fmt == FMT0 {
		switch chunk.MessageType {
		case SET_CHUNK_SIZE:
			// return newSetChunkSizeMessage(rtmp, chunk)
		case ABORT_MESSAGE:
		case ACKNOWLEDGEMENT:
		case USER_CONTROL_MESSAGE:
		case WINDOW_ACKNOWLEDGEMENT_SIZE:
		case SET_PEER_BANDWIDTH:
		// case XXX:

		case AUDIO_MESSAGE:
		case VIDEO_MESSAGE:

		case DATA_MESSAGE_AMF0, DATA_MESSAGE_AMF3:
		case SHARE_OBJECT_MESSAGE_AMF0, SHARE_OBJECT_MESSAGE_AMF3:
		case COMMAND_MESSAGE_AMF0, COMMAND_MESSAGE_AMF3:
			message, err = parseCommandMessage(rtmp, chunk)
		default:
			return errors.New("invalid message type")
		}

		if err != nil {
			return err
		}

		mb := &MessageBase{
			rtmp:                rtmp,
			messageTime:         chunk.MessageTimeStamp,
			messageLength:       chunk.MessageLength,
			messageLengthRemain: 0,
			messageType:         chunk.MessageType,
			messageStreamID:     chunk.MessageStreamID,
		}
		if mb.messageLength > rtmp.chunkSize {
			mb.messageLengthRemain = mb.messageLength - rtmp.chunkSize
		}
		message.SetInfo(mb)
		rtmp.message[mb.messageStreamID] = message
	} else {
		var ok bool
		message, ok = rtmp.message[chunk.MessageStreamID]
		if !ok {
			return errors.New("invalid chunk format")
		} else {
			mb := message.GetInfo()
			switch chunk.Fmt {
			case FMT1, FMT2:
				mb.messageTimeDelta = chunk.MessageTimeStamp
				message.SetInfo(mb)
				rtmp.message[mb.messageStreamID] = message
			case FMT3: //do nothing
			default:
				return errors.New("invalid chunk format")
			}
			err = message.Update(chunk)
			if err != nil {
				return err
			}
		}
	}

	return message.Do()
}
