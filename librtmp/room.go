package librtmp

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/SmartBrave/GGmpeg/libflv"
	"github.com/SmartBrave/utils_sb/broadcast"
	"github.com/SmartBrave/utils_sb/easyerrors"
	"github.com/SmartBrave/utils_sb/easyio"
)

type Room struct {
	RoomID    string
	Publisher *RTMP //TODO: support multi publisher
	//Players   sync.Map     //peer ip+port, rtmp conn
	Meta          libflv.Tag //TODO: RWMutex
	AudioSeq      libflv.Tag //TODO: RWMutex
	VideoSeq      libflv.Tag //TODO: RWMutex
	MetaMutex     sync.RWMutex
	AudioSeqMutex sync.RWMutex
	VideoSeqMutex sync.RWMutex
	GOP           *broadcast.Broadcast
}

//NOTE: the room must be created by publisher
func NewRoom(rtmp *RTMP, roomID string) *Room {
	r := &Room{
		RoomID:    roomID,
		Publisher: rtmp,
		//Players:   sync.Map{},
		GOP: broadcast.NewBroadcast(),
	}
	return r
}

//player join the room
func (room *Room) RTMPJoin(rtmp *RTMP) {
	//room.Players.Store(rtmp.peer, rtmp)
	gopReader := broadcast.NewBroadcastReader(room.GOP)

	go func() {
		var err1, err2, err3 error
		room.MetaMutex.RLock()
		dm := NewDataMessage(MessageBase{
			rtmp:        rtmp,
			messageTime: room.Meta.GetTagInfo().TimeStamp,
			// messageLength:    room.Meta.GetTagInfo().DataSize,
			messageType:     MessageType(room.Meta.GetTagInfo().TagType),
			messageStreamID: 0,
		}, room.Meta)
		room.MetaMutex.RUnlock()
		err1 = dm.Send()

		room.VideoSeqMutex.RLock()
		vm := NewVideoMessage(MessageBase{
			rtmp:            rtmp,
			messageTime:     room.VideoSeq.GetTagInfo().TimeStamp,
			messageLength:   room.VideoSeq.GetTagInfo().DataSize,
			messageType:     MessageType(room.VideoSeq.GetTagInfo().TagType),
			messageStreamID: 0,
		}, room.VideoSeq)
		room.VideoSeqMutex.RUnlock()
		err2 = vm.Send()

		room.AudioSeqMutex.RLock()
		am := NewAudioMessage(MessageBase{
			rtmp:            rtmp,
			messageTime:     room.AudioSeq.GetTagInfo().TimeStamp,
			messageLength:   room.AudioSeq.GetTagInfo().DataSize,
			messageType:     MessageType(room.AudioSeq.GetTagInfo().TagType),
			messageStreamID: 0,
		}, room.AudioSeq)
		room.AudioSeqMutex.RUnlock()
		err3 = am.Send()

		err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
		if err != nil {
			fmt.Println("send meta and av seq error:", err)
		}

		for {
			p, alive := gopReader.Read()
			if !alive {
				fmt.Println("the publisher had been exit.")
				break
			}
			fmt.Printf("read package from gop, now:%v\n", time.Now())
			tag := p.(libflv.Tag)
			mb := MessageBase{
				rtmp:            rtmp,
				messageTime:     tag.GetTagInfo().TimeStamp,
				messageLength:   tag.GetTagInfo().DataSize,
				messageType:     MessageType(tag.GetTagInfo().TagType),
				messageStreamID: 0,
			}
			if audioTag, oka := tag.(*libflv.AudioTag); oka {
				fmt.Printf("[gop send audio] message time:%d\n", mb.messageTime)
				err = NewAudioMessage(mb, audioTag).Send()
			} else if videoTag, okv := tag.(*libflv.VideoTag); okv {
				fmt.Printf("[gop send video] message time:%d, componsition time:%d\n", mb.messageTime, videoTag.CompositionTime)
				err = NewVideoMessage(mb, videoTag).Send()
			} else {
				//XXX
			}
			if err != nil {
				fmt.Println("send data error:", err)
			}
		}
	}()
}

func (room *Room) FLVJoin(writer easyio.EasyWriter) {
	writer.Write([]byte{0x46, 0x4c, 0x56, 0x01, 0x05, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00})
	gopReader := broadcast.NewBroadcastReader(room.GOP)
	var writedSize uint32 = 11
	var b []byte
	writedSizeByte := make([]byte, 4, 4)
	go func() {
		//room.MetaMutex.RLock()
		//b = room.Meta.Marshal()
		//room.MetaMutex.RUnlock()
		//writer.Write(b)
		//writedSize += uint32(len(b))
		//binary.BigEndian.PutUint32(writedSizeByte, writedSize)
		//writer.Write(writedSizeByte)

		room.VideoSeqMutex.RLock()
		b = room.VideoSeq.Marshal()
		room.VideoSeqMutex.RUnlock()
		writer.Write(b)
		writedSize += uint32(len(b))
		binary.BigEndian.PutUint32(writedSizeByte, writedSize)
		writer.Write(writedSizeByte)

		room.AudioSeqMutex.RLock()
		b = room.AudioSeq.Marshal()
		room.AudioSeqMutex.RUnlock()
		writer.Write(b)
		writedSize += uint32(len(b))
		binary.BigEndian.PutUint32(writedSizeByte, writedSize)
		writer.Write(writedSizeByte)
		for {
			p, alive := gopReader.Read()
			if !alive {
				fmt.Println("the publisher had been exit.")
				break
			}
			tag := p.(libflv.Tag)
			writedSize += writeFLVTag(tag, writer)
			binary.BigEndian.PutUint32(writedSizeByte, writedSize)
			writer.Write(writedSizeByte)
		}
	}()
}

func writeFLVTag(tag libflv.Tag, flvWriter easyio.EasyWriter) (n uint32) {
	b := tag.Marshal()
	flvWriter.Write(b)
	return uint32(len(b))
}
