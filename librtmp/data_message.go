package librtmp

import (
	"fmt"

	"github.com/sbraveyoung/GGmpeg/libflv"
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
	chunkSize := dm.rtmp.ownMaxChunkSize
	for i := 0; ; i++ {
		lIndex := i * chunkSize
		if lIndex >= len(dm.messagePayload) {
			break
		}
		rIndex := lIndex + chunkSize
		if rIndex > len(dm.messagePayload) {
			rIndex = len(dm.messagePayload)
		}
		fmtType := FMT0
		if i != 0 {
			fmtType = FMT3
		}
		if sendErr := NewChunk(DATA_MESSAGE_AMF0, uint32(len(dm.messagePayload)), dm.messageTime, fmtType, csidData, dm.messagePayload[lIndex:rIndex]).Send(dm.rtmp); sendErr != nil {
			return sendErr
		}
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
	if err != nil {
		return err
	}
	dm.metaTag.DataSize = uint32(len(dm.metaTag.Data()))
	return nil
}

func (dm *DataMessage) Do() (err error) {
	if dm.rtmp.room == nil {
		return nil
	}
	dm.rtmp.room.setMeta(dm.metaTag)
	dm.rtmp.room.GOP.WriteMeta(dm.metaTag)
	fmt.Printf("write packet data :%+v\n", dm.metaTag)

	return nil
}
