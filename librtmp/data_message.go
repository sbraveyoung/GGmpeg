package librtmp

import (
	"fmt"

	"github.com/SmartBrave/GGmpeg/libflv"
)

type DataMessage struct {
	MessageBase
	metaTag *libflv.MetaTag
}

func NewDataMessage(mb MessageBase, fields ...interface{}) (dm *DataMessage) {
	dm = &DataMessage{
		MessageBase: mb,
	}

	if len(fields) == 1 {
		var ok bool
		if dm.metaTag, ok = fields[0].(*libflv.MetaTag); !ok {
			//TODO
		} else {
			dm.messagePayload = dm.metaTag.Marshal()
		}
	}
	return dm
}

func (dm *DataMessage) Send() (err error) {
	for i := 0; i >= 0; i++ {
		format := FMT0
		if i != 0 {
			format = FMT3
		}

		lIndex := i * int(dm.rtmp.ownMaxChunkSize)
		rIndex := (i + 1) * int(dm.rtmp.ownMaxChunkSize)
		if rIndex > len(dm.messagePayload) {
			rIndex = len(dm.messagePayload)
			i = -2
		}
		NewChunk(DATA_MESSAGE_AMF0, uint32(len(dm.messagePayload)), dm.messageTime, format, 6, dm.messagePayload[lIndex:rIndex]).Send(dm.rtmp)
	}
	return nil
}

func (dm *DataMessage) Parse() (err error) {
	dm.metaTag, err = libflv.ParseMetaTag(libflv.TagBase{
		TagType:   libflv.SCRIPT_DATA_TAG,
		DataSize:  dm.messageLength,
		TimeStamp: dm.messageTime,
		StreamID:  0,
	}, dm.amf, dm.messagePayload)
	fmt.Printf("[meta] data_message:%+v\n", dm.metaTag)
	return err
}

func (dm *DataMessage) Do() (err error) {
	dm.rtmp.room.GOP.WriteMeta(dm.metaTag)
	fmt.Printf("write packet data :%+v\n", dm.metaTag)

	return nil
}
