package librtmp

import (
	"fmt"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/SmartBrave/Athena/easyio"
	"github.com/SmartBrave/GGmpeg/libflv"
)

type Room struct {
	RoomID    string
	Publisher *RTMP //TODO: support multi publisher
	//Players   sync.Map     //peer ip+port, rtmp conn
	GOP *broadcast.Broadcast
}

//NOTE: the room must be created by publisher
func NewRoom(rtmp *RTMP, roomID string) *Room {
	r := &Room{
		RoomID:    roomID,
		Publisher: rtmp,
		//Players:   sync.Map{},
		GOP: broadcast.NewBroadcast(3),
	}
	return r
}

//player join the room
func (room *Room) RTMPJoin(rtmp *RTMP) {
	//room.Players.Store(rtmp.peer, rtmp)

	//XXX: using goroutine is unnecessary?
	go func() {
		var err error
		gopReader := broadcast.NewBroadcastReader(room.GOP)
		for {
			p, alive := gopReader.Read()
			if !alive {
				fmt.Println("the publisher had been exit.")
				break
			}
			tag := p.(libflv.Tag)
			fmt.Printf("read packet from gop, now:%v, tag:%+v\n", time.Now(), tag)
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
				fmt.Printf("[gop send video] message time:%d, componsition time:%d\n", mb.messageTime, videoTag.Cts)
				err = NewVideoMessage(mb, videoTag).Send()
			} else if dataTag, okd := tag.(*libflv.MetaTag); okd {
				err = NewDataMessage(mb, dataTag).Send()
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
