package rtmp

import (
	"fmt"
	"sync"

	"github.com/SmartBrave/GGmpeg/flv"
	"github.com/SmartBrave/utils/easyerrors"
)

type Room struct {
	RoomID    string
	Publisher *RTMP    //TODO: support multi publisher
	Players   sync.Map //peer ip+port, rtmp conn
	Meta      *DataMessage
	AudioSeq  flv.Tag
	VideoSeq  flv.Tag
	GOP       []flv.Tag
	ch        chan flv.Tag
}

func NewRoom(roomID string) *Room {
	r := &Room{
		RoomID:  roomID,
		Players: sync.Map{},
		GOP:     make([]flv.Tag, 0, 1024),
		ch:      make(chan flv.Tag, 1024),
	}
	go r.Transmit()
	return r
}

func (r *Room) Transmit() {
	for {
		tag := <-r.ch
		// <-r.ch

		r.Players.Range(func(key, value interface{}) bool {
			// peer, _ := key.(string)
			rtmp, _ := value.(*RTMP)

			if !rtmp.start {
				var err1, err2, err3 error
				err1 = NewDataMessage(MessageBase{
					rtmp:        rtmp,
					messageTime: r.Meta.messageTime,
					// messageLength:    info.DataSize,
					messageType:     MessageType(DATA_MESSAGE_AMF0),
					messageStreamID: 0,
				}, r.Meta.FirstField, r.Meta.SecondField, r.Meta.MetaData).Send()
				err2 = NewVideoMessage(MessageBase{
					rtmp:            rtmp,
					messageTime:     r.VideoSeq.GetTagInfo().TimeStamp,
					messageLength:   r.VideoSeq.GetTagInfo().DataSize,
					messageType:     MessageType(r.VideoSeq.GetTagInfo().TagType),
					messageStreamID: 0,
				}, "video sequence", -1, r.VideoSeq).Send()
				// err3 = NewAudioMessage(MessageBase{
				// rtmp:             rtmp,
				// messageTime:      r.AudioSeq.GetTagInfo().TimeStamp,
				// messageLength:    r.AudioSeq.GetTagInfo().DataSize,
				// messageType:      MessageType(r.AudioSeq.GetTagInfo().TagType),
				// messageStreamID:  0,
				// }, r.AudioSeq).Send()
				err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
				if err != nil {
					fmt.Println("send meta and av seq error:", err)
				}
				for index, tag := range r.GOP {
					if audioTag, oka := tag.(*flv.AudioTag); false && oka {
						err = NewAudioMessage(MessageBase{
							rtmp:            rtmp,
							messageTime:     audioTag.GetTagInfo().TimeStamp,
							messageLength:   audioTag.GetTagInfo().DataSize,
							messageType:     MessageType(audioTag.GetTagInfo().TagType),
							messageStreamID: 0,
						}, audioTag).Send()
					} else if videoTag, okv := tag.(*flv.VideoTag); okv {
						err = NewVideoMessage(MessageBase{
							rtmp:            rtmp,
							messageTime:     videoTag.GetTagInfo().TimeStamp,
							messageLength:   videoTag.GetTagInfo().DataSize,
							messageType:     MessageType(videoTag.GetTagInfo().TagType),
							messageStreamID: 0,
						}, "gop", index, videoTag).Send()
					}
					if err != nil {
						fmt.Println("send video data error:", err)
					}
				}
				rtmp.start = true
				return true
			}

			mb := MessageBase{
				rtmp:            rtmp,
				messageTime:     tag.GetTagInfo().TimeStamp,
				messageLength:   tag.GetTagInfo().DataSize,
				messageType:     MessageType(tag.GetTagInfo().TagType),
				messageStreamID: 0,
			}
			var err error
			if audioTag, oka := tag.(*flv.AudioTag); false && oka {
				err = NewAudioMessage(mb, audioTag).Send()
			} else if videoTag, okv := tag.(*flv.VideoTag); okv {
				err = NewVideoMessage(mb, "packet", -3, videoTag).Send()
			}
			if err != nil {
				fmt.Println("send video data error:", err)
			}
			return true
		})
	}
}
