package rtmp

import (
	"fmt"

	amf_pkg "github.com/SmartBrave/GGmpeg/rtmp/amf"
	"github.com/SmartBrave/utils/easyerrors"
	"github.com/pkg/errors"
)

type MessageBase struct {
	rtmp             *RTMP
	messageTime      uint32
	messageTimeDelta uint32
	messageLength    int
	messageType      MessageType
	messageStreamID  uint32
	amf              amf_pkg.AMF
	messagePayload   []byte //TODO: maybe using easyio.EasyReadWriter
}

func (mb *MessageBase) GetInfo() *MessageBase {
	return mb
}

func (mb *MessageBase) Update(mbNew *MessageBase) {
	//do not update other fields
	mb.messageTimeDelta = mbNew.messageTimeDelta
}

func (mb *MessageBase) Append(chunk *Chunk) {
	mb.messagePayload = append(mb.messagePayload, chunk.Payload...)
}

func (mb *MessageBase) Done() int {
	fmt.Printf("done? messageLength:%d, len(payload):%d\n", mb.messageLength, len(mb.messagePayload))
	return mb.messageLength - len(mb.messagePayload)
}

type Message interface {
	Append(*Chunk)
	Done() int
	GetInfo() *MessageBase
	Update(*MessageBase)

	Parse() error
	Do() error
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

func ParseMessage(rtmp *RTMP) (err error) {
	var chunk *Chunk
	var message Message
	// var ok bool

	//read message payload from many chunks
	for {
		chunk, err = ParseChunk(rtmp, message)
		if err != nil {
			return err
		}

		// message, ok = rtmp.message[chunk.MessageStreamID]
		// if !ok {
		if message == nil {
			mb := MessageBase{
				rtmp:        rtmp,
				messageTime: chunk.MessageTimeStamp,
				// messageTimeDelta:0
				messageLength:   chunk.MessageLength,
				messageType:     chunk.MessageType,
				messageStreamID: chunk.MessageStreamID,
				amf:             amf_pkg.AMF0,
				messagePayload:  make([]byte, 0, chunk.MessageLength),
			}

			switch chunk.MessageType {
			case SET_CHUNK_SIZE:
				// return newSetChunkSizeMessage(rtmp, chunk)
			case ABORT_MESSAGE:
			case ACKNOWLEDGEMENT:
			case USER_CONTROL_MESSAGE:
			case WINDOW_ACKNOWLEDGEMENT_SIZE:
			case SET_PEER_BANDWIDTH:

			case AUDIO_MESSAGE:
			case VIDEO_MESSAGE:

			case DATA_MESSAGE_AMF3:
				// mb.amf = amf_pkg.AMF3
				fallthrough
			case DATA_MESSAGE_AMF0:
				message = &DataMessage{
					MessageBase: mb,
				}
			case SHARE_OBJECT_MESSAGE_AMF3:
				// mb.amf = amf_pkg.AMF3
				fallthrough
			case SHARE_OBJECT_MESSAGE_AMF0:
			case COMMAND_MESSAGE_AMF3:
				// mb.amf = amf_pkg.AMF3
				fallthrough
			case COMMAND_MESSAGE_AMF0:
				// message, err = parseCommandMessage(rtmp, chunk)
				message = &CommandMessage{
					MessageBase: mb,
				}
			default:
				return errors.New("invalid message type")
			}
		}
		// } else {
		// mb := message.GetInfo()
		// switch chunk.Fmt {
		// case FMT1, FMT2:
		// mb.messageTimeDelta = chunk.MessageTimeStamp
		// message.Update(mb)
		// rtmp.message[mb.messageStreamID] = message
		// case FMT3: //do nothing
		// default:
		// return errors.New("invalid chunk format")
		// }
		// }

		message.Append(chunk)
		// rtmp.message[chunk.MessageStreamID] = message
		if message.Done() == 0 {
			break
		}
	}

	return easyerrors.HandleMultiError(easyerrors.Simple(), message.Parse(), message.Do())
}
