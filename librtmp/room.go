package librtmp

import (
	"fmt"
	"sync"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libflv"
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

	//XXX: using goroutine is unnecessary?
	go func() {
		gopReader := broadcast.NewBroadcastReader(room.GOP)
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
				fmt.Printf("[gop send audio] message time:%d, dataSize:%d\n", mb.messageTime, mb.messageLength)
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
	var b []byte

	room.MetaMutex.RLock()
	b = libflv.FLVWrite(room.Meta)
	room.MetaMutex.RUnlock()
	writer.Write(b)

	room.VideoSeqMutex.RLock()
	b = libflv.FLVWrite(room.VideoSeq)
	room.VideoSeqMutex.RUnlock()
	writer.Write(b)

	room.AudioSeqMutex.RLock()
	b = libflv.FLVWrite(room.AudioSeq)
	room.AudioSeqMutex.RUnlock()
	writer.Write(b)
	for {
		p, alive := gopReader.Read()
		if !alive { //XXX: `if !alive && p==nil` is better?
			fmt.Println("the publisher had been exit.")
			break
		}
		writer.Write(libflv.FLVWrite(p.(libflv.Tag)))
	}
}

func (room *Room) HLSJoin(writer easyio.EasyWriter) {
	//XXX: remove to libhls
	writer.Write([]byte(fmt.Sprintf(`#EXTM3U
#EXT-X-VERSION:3
#EXT-X-ALLOW-CACHE:YES
#EXT-X-TARGETDURATION:%d
#EXT-X-MEDIA-SEQUENCE:%d`, 2, 35)))
}
