package rtmp

// import "net"

type Message interface {
}

// func NewMessage(conn net.Conn) (message *Message) {
//  TODO
// }

/*
 * MessageType 1-7
   * 1: set chunk size
   * 2: abort message
   * 3: acknowledgement
   * 4: user control message
   * 5: window acknowledgement size
   * 6: set peer bandwidth
   * 7: TODO is used between edge server and origin server
 * MessageStreamID: 0
 * ChunkStreamID: 2
*/
type MessageType uint8

const (
	SET_CHUNK_SIZE MessageType = iota + 1
	ABORT_MESSAGE
	ACKNOWLEDGEMENT
	USER_CONTROL_MESSAGE
	WINDOW_ACKNOWLEDGEMENT_SIZE
	SET_PEER_BANDWIDTH
	XXX
)

type MessageHeader struct {
	MessageType     uint8
	PayloadLength   uint32 //3bytes
	TimeStamp       uint32
	MessageStreamID uint32 //3bytes
}

type ControlMessage interface {
	Do() error
}

type MessageSetChunkSize struct {
	MessageHeader
	NewChunkSize uint32
}

func (mscs *MessageSetChunkSize) Do() error {
	//TODO
	return nil
}

type MessageAbort struct {
	MessageHeader
	ChunkStreamID uint32
}

func (ma *MessageAbort) Do() error {
	//TODO
	return nil
}

type MessageAcknowledgement struct {
	MessageHeader
	SequenceNumber uint32
}

func (ma *MessageAcknowledgement) Do() error {
	//TODO
	return nil
}

type MessageUserControl struct {
	MessageHeader
	EventType uint16
	EventData []byte
}

func (muc *MessageUserControl) Do() error {
	//TODO
	return nil
}

type MessageWindowAcknowledgeSize struct {
	MessageHeader
	AcknowledgementWindowSize uint32
}

func (mwas *MessageWindowAcknowledgeSize) Do() error {
	//TODO
	return nil
}

type LimitType uint8

const (
	HARD    LimitType = 0
	SOFT    LimitType = 1
	DYNAMIC LimitType = 2
)

type MessageSetPeerBandwidth struct {
	MessageHeader
	AcknowledgementWindowSize uint32
	LimitType                 LimitType
}

func (mspb *MessageSetPeerBandwidth) Do() error {
	//TODO
	return nil
}
