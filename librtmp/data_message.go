package librtmp

import (
	"bytes"
	"fmt"
	"io"

	"github.com/SmartBrave/GGmpeg/libamf"
	"github.com/SmartBrave/GGmpeg/libflv"
	"github.com/SmartBrave/utils_sb/easyerrors"
	"github.com/SmartBrave/utils_sb/easyio"
	"github.com/fatih/structs"
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
		}
	}
	return dm
}

func (dm *DataMessage) Send() (err error) {
	//TODO
	buf := bytes.NewBuffer([]byte{})
	writer := easyio.NewEasyWriter(buf)
	amf := libamf.AMF0

	err1 := amf.Encode(writer, dm.metaTag.FirstField)
	err2 := amf.Encode(writer, dm.metaTag.SecondField)
	err3 := amf.Encode(writer, structs.Map(dm.metaTag))
	err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
	if err != nil {
		fmt.Println("HandleMultiError error:", err)
		return err
	}

	var b []byte
	b, err = io.ReadAll(buf)
	if err != nil {
		return err
	}

	for i := 0; i >= 0; i++ {
		format := FMT0
		if i != 0 {
			format = FMT3
		}

		lIndex := i * int(dm.rtmp.ownMaxChunkSize)
		rIndex := (i + 1) * int(dm.rtmp.ownMaxChunkSize)
		if rIndex > len(b) {
			rIndex = len(b)
			i = -2
		}
		NewChunk(DATA_MESSAGE_AMF0, uint32(len(b)), dm.messageTime, format, 6, b[lIndex:rIndex]).Send(dm.rtmp)
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
	return err
}

func (dm *DataMessage) Do() (err error) {
	dm.rtmp.room.MetaMutex.Lock()
	dm.rtmp.room.Meta = dm.metaTag
	dm.rtmp.room.MetaMutex.Unlock()
	return nil
}
