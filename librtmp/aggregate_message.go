package librtmp

import (
	"encoding/binary"
	"fmt"

	"github.com/SmartBrave/GGmpeg/libamf"
)

// AggregateMessage (message type 22) is a container whose payload is a
// tightly packed list of FLV-tag-style sub-messages. Each sub-message
// carries its own type/length/timestamp/stream-id header, followed by
// the inner body, followed by a 4-byte back-pointer. See RTMP 1.0
// §5.4.2 and FLV v10.1 §E.4.1.
//
// Concrete sub-message types we care about are audio (8), video (9),
// and script-data (18) — we route each to the existing message
// implementation so parsing/GOP-bookkeeping reuses one code path.
type AggregateMessage struct {
	MessageBase
}

func NewAggregateMessage(mb MessageBase) *AggregateMessage {
	return &AggregateMessage{MessageBase: mb}
}

func (am *AggregateMessage) Parse() error { return nil }

func (am *AggregateMessage) Send() error {
	//Server-only today; we never originate aggregate messages.
	return fmt.Errorf("AggregateMessage.Send not supported")
}

func (am *AggregateMessage) Do() error {
	payload := am.messagePayload
	//Aggregate timestamps are relative to the aggregate's own timestamp
	//(§5.4.2: "the timestamp of the first sub-message is used as an
	//offset"). Tracks the delta between the first sub-message stamp and
	//the container stamp so we can rebase subsequent stamps.
	var firstSubTS uint32
	gotFirst := false

	for off := 0; off+11 <= len(payload); {
		subType := MessageType(payload[off])
		subLen := uint32(payload[off+1])<<16 | uint32(payload[off+2])<<8 | uint32(payload[off+3])
		ts := uint32(payload[off+4])<<16 | uint32(payload[off+5])<<8 | uint32(payload[off+6]) | uint32(payload[off+7])<<24
		//stream id is little-endian 3 bytes (payload[off+8..10]); always 0 in practice, ignore

		headerEnd := off + 11
		bodyEnd := headerEnd + int(subLen)
		if bodyEnd > len(payload) {
			return fmt.Errorf("aggregate sub-message truncated: need %d, have %d", bodyEnd, len(payload))
		}

		if !gotFirst {
			firstSubTS = ts
			gotFirst = true
		}
		//Rebase to the container's timestamp.
		rebased := am.messageTime + (ts - firstSubTS)

		subBase := MessageBase{
			rtmp:            am.rtmp,
			messageTime:     rebased,
			messageLength:   subLen,
			messageType:     subType,
			messageStreamID: am.messageStreamID,
			amf:             libamf.AMF0,
			messagePayload:  payload[headerEnd:bodyEnd],
		}

		var sub Message
		switch subType {
		case AUDIO_MESSAGE:
			sub = NewAudioMessage(subBase)
		case VIDEO_MESSAGE:
			sub = NewVideoMessage(subBase)
		case DATA_MESSAGE_AMF0, DATA_MESSAGE_AMF3:
			sub = NewDataMessage(subBase)
		default:
			//Skip unsupported sub-messages rather than abort.
			sub = nil
		}
		if sub != nil {
			if err := sub.Parse(); err != nil {
				return fmt.Errorf("aggregate sub Parse: %w", err)
			}
			if err := sub.Do(); err != nil {
				return fmt.Errorf("aggregate sub Do: %w", err)
			}
		}

		//4-byte back pointer follows the body and equals the total
		//sub-message length (11 header + subLen body). Skip over it.
		off = bodyEnd + 4
		_ = binary.BigEndian // kept for future validation of the back pointer
	}
	return nil
}
