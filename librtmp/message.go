package librtmp

import (
	"fmt"

	"github.com/SmartBrave/GGmpeg/libamf"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/pkg/errors"
)

type MessageBase struct {
	rtmp            *RTMP
	messageTime     uint32
	messageLength   uint32
	messageType     MessageType
	messageStreamID uint32
	amf             libamf.AMF
	messagePayload  []byte //TODO: maybe using easyio.EasyReadWriter
}

func (mb *MessageBase) GetInfo() *MessageBase {
	return mb
}

func (mb *MessageBase) Update(time uint32) {
	//do not update other fields
	mb.messageTime = time
}

func (mb *MessageBase) Append(chunk *Chunk) {
	mb.messagePayload = append(mb.messagePayload, chunk.Payload...)
}

func (mb *MessageBase) Remain() uint32 {
	// fmt.Printf("done? messageLength:%d, len(payload):%d\n", mb.messageLength, len(mb.messagePayload))
	return mb.messageLength - uint32(len(mb.messagePayload))
}

func (mb *MessageBase) Done() bool {
	return mb.Remain() == 0
}

type Message interface {
	Append(*Chunk)
	Remain() uint32
	Done() bool
	GetInfo() *MessageBase
	Update(uint32)

	//Parse() parse binary data that receive from peer
	Parse() error
	//when receive the message, Do() operator fields in RTMP belongs to this message, and send response to peer
	Do() error
	//Send() post the message to peer which generated from NewXXX()
	Send() error
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

	//aggregate message (RTMP 1.0 §5.4.2 / FLV v10.1 §E.4.1)
	AGGREGATE_MESSAGE MessageType = 22
)

func ParseMessage(rtmp *RTMP) (err error) {
	var chunk *Chunk
	var message Message

	//read message payload from many chunks
	for {
		chunk, err = ParseChunk(rtmp, message)
		if err != nil {
			return err
		}

		if message == nil {
			mb := MessageBase{
				rtmp:            rtmp,
				messageTime:     chunk.MessageTimeStamp,
				messageLength:   chunk.MessageLength,
				messageType:     chunk.MessageType,
				messageStreamID: chunk.MessageStreamID,
				amf:             libamf.AMF0,
				messagePayload:  make([]byte, 0, chunk.MessageLength),
			}

			switch chunk.MessageType {
			case SET_CHUNK_SIZE:
				message = NewSetChunkSizeMessage(mb)
			case ABORT_MESSAGE:
				message = NewAbortMessage(mb)
			case ACKNOWLEDGEMENT:
				message = NewAcknowledgeMessage(mb)
			case USER_CONTROL_MESSAGE:
				message = NewUserControlMessage(mb)
			case WINDOW_ACKNOWLEDGEMENT_SIZE:
				message = NewWindowAcknowledgeSizeMessage(mb)
			case SET_PEER_BANDWIDTH:
				message = NewSetPeerBandWidthMessage(mb)

			case AUDIO_MESSAGE:
				message = NewAudioMessage(mb)
			case VIDEO_MESSAGE:
				message = NewVideoMessage(mb)

			case AGGREGATE_MESSAGE:
				message = NewAggregateMessage(mb)

			case DATA_MESSAGE_AMF3:
				// mb.amf = libamf.AMF3
				fallthrough
			case DATA_MESSAGE_AMF0:
				message = NewDataMessage(mb)

			case SHARE_OBJECT_MESSAGE_AMF3:
				// mb.amf = libamf.AMF3
				fallthrough
			case SHARE_OBJECT_MESSAGE_AMF0:
				//Shared objects are a flash-era state-sync channel we
				//don't implement. Drain the payload and move on rather
				//than falling through into the default error branch.
				message = NewDiscardMessage(mb)

			case COMMAND_MESSAGE_AMF3:
				// mb.amf = libamf.AMF3
				fallthrough
			case COMMAND_MESSAGE_AMF0:
				message = NewCommandMessage(mb)

			default:
				return errors.New("invalid message type")
			}
		} else {
			message.Update(chunk.MessageTimeStamp)
		}

		message.Append(chunk)
		if message.Done() {
			break
		}
	}

	//Track inbound bytes so we can send Window Acknowledgement messages
	//on schedule. The sequence number is the cumulative count received.
	rtmp.bytesReceived += message.GetInfo().messageLength
	if rtmp.peerWindowAckSize > 0 && rtmp.bytesReceived-rtmp.lastAcked >= rtmp.peerWindowAckSize {
		ack := NewAcknowledgeMessage(MessageBase{rtmp: rtmp}, rtmp.bytesReceived)
		if sendErr := ack.Send(); sendErr == nil {
			rtmp.lastAcked = rtmp.bytesReceived
		} else {
			fmt.Println("send acknowledgement error:", sendErr)
		}
	}

	return easyerrors.HandleMultiError(easyerrors.Simple(), message.Parse(), message.Do())
}

// DiscardMessage consumes (and drops) a message whose type we don't
// implement — used for shared-object and other seldom-seen types so
// ParseMessage doesn't abort the connection.
type DiscardMessage struct {
	MessageBase
}

func NewDiscardMessage(mb MessageBase) *DiscardMessage {
	return &DiscardMessage{MessageBase: mb}
}

func (dm *DiscardMessage) Parse() error { return nil }
func (dm *DiscardMessage) Do() error    { return nil }
func (dm *DiscardMessage) Send() error  { return nil }
