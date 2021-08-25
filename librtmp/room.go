package librtmp

import (
	"fmt"
	"sync"

	"github.com/SmartBrave/GGmpeg/libflv"
	"github.com/SmartBrave/utils_sb/easyerrors"
	"github.com/SmartBrave/utils_sb/gop"
)

type Room struct {
	RoomID    string
	Publisher *RTMP    //TODO: support multi publisher
	Players   sync.Map //peer ip+port, rtmp conn
	Meta      *DataMessage
	AudioSeq  libflv.Tag
	VideoSeq  libflv.Tag
	GOP       *gop.GOP
}

//NOTE: the room must be created by publisher
func NewRoom(rtmp *RTMP, roomID string) *Room {
	r := &Room{
		RoomID:    roomID,
		Publisher: rtmp,
		Players:   sync.Map{},
		GOP:       gop.NewGOP(),
	}
	go r.Transmit()
	return r
}

//player join the room
func (room *Room) Join(rtmp *RTMP) {
	room.Players.Store(rtmp.peer, rtmp)
	rtmp.gopReader = gop.NewGOPReader(room.GOP)

	go func() {
		var err1, err2, err3 error
		err1 = NewDataMessage(MessageBase{
			rtmp:        rtmp,
			messageTime: room.Meta.messageTime,
			// messageLength:    info.DataSize,
			messageType:     MessageType(DATA_MESSAGE_AMF0),
			messageStreamID: 0,
		}, room.Meta.FirstField, room.Meta.SecondField, room.Meta.MetaData).Send()
		err2 = NewVideoMessage(MessageBase{
			rtmp:            rtmp,
			messageTime:     room.VideoSeq.GetTagInfo().TimeStamp,
			messageLength:   room.VideoSeq.GetTagInfo().DataSize,
			messageType:     MessageType(room.VideoSeq.GetTagInfo().TagType),
			messageStreamID: 0,
		}, room.VideoSeq).Send()
		err3 = NewAudioMessage(MessageBase{
			rtmp:            rtmp,
			messageTime:     room.AudioSeq.GetTagInfo().TimeStamp,
			messageLength:   room.AudioSeq.GetTagInfo().DataSize,
			messageType:     MessageType(room.AudioSeq.GetTagInfo().TagType),
			messageStreamID: 0,
		}, room.AudioSeq).Send()
		err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
		if err != nil {
			fmt.Println("send meta and av seq error:", err)
		}

		for {
			p, alive := rtmp.gopReader.Read()
			if !alive {
				fmt.Println("the publisher had been exit.")
				break
			}
			tag := p.(libflv.Tag)
			mb := MessageBase{
				rtmp:            rtmp,
				messageTime:     tag.GetTagInfo().TimeStamp,
				messageLength:   tag.GetTagInfo().DataSize,
				messageType:     MessageType(tag.GetTagInfo().TagType),
				messageStreamID: 0,
			}
			if audioTag, oka := tag.(*libflv.AudioTag); oka {
				err = NewAudioMessage(mb, audioTag).Send()
			} else if videoTag, okv := tag.(*libflv.VideoTag); okv {
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

func (r *Room) Transmit() {
	for {
		r.Players.Range(func(key, value interface{}) bool {
			// peer, _ := key.(string)
			//rtmp, _ := value.(*RTMP)
			//if rtmp.playType == "" || rtmp.playType == "rtmp" {
			//} else if rtmp.playType == "flv" {
			//	if !rtmp.start {
			//		var err1, err2, err3 error
			//		err1 = NewDataMessage(MessageBase{
			//			rtmp:        rtmp,
			//			messageTime: r.Meta.messageTime,
			//			// messageLength:    info.DataSize,
			//			messageType:     MessageType(DATA_MESSAGE_AMF0),
			//			messageStreamID: 0,
			//		}, r.Meta.FirstField, r.Meta.SecondField, r.Meta.MetaData).Send()
			//		err2 = NewVideoMessage(MessageBase{
			//			rtmp:            rtmp,
			//			messageTime:     r.VideoSeq.GetTagInfo().TimeStamp,
			//			messageLength:   r.VideoSeq.GetTagInfo().DataSize,
			//			messageType:     MessageType(r.VideoSeq.GetTagInfo().TagType),
			//			messageStreamID: 0,
			//		}, r.VideoSeq).Send()
			//		err3 = NewAudioMessage(MessageBase{
			//			rtmp:            rtmp,
			//			messageTime:     r.AudioSeq.GetTagInfo().TimeStamp,
			//			messageLength:   r.AudioSeq.GetTagInfo().DataSize,
			//			messageType:     MessageType(r.AudioSeq.GetTagInfo().TagType),
			//			messageStreamID: 0,
			//		}, r.AudioSeq).Send()
			//		err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
			//		if err != nil {
			//			fmt.Println("send meta and av seq error:", err)
			//		}
			//		for index, tag := range r.GOP {
			//			if audioTag, oka := tag.(*libflv.AudioTag); oka {
			//				err = NewAudioMessage(MessageBase{
			//					rtmp:            rtmp,
			//					messageTime:     audioTag.GetTagInfo().TimeStamp,
			//					messageLength:   audioTag.GetTagInfo().DataSize,
			//					messageType:     MessageType(audioTag.GetTagInfo().TagType),
			//					messageStreamID: 0,
			//				}, audioTag).Send()
			//			} else if videoTag, okv := tag.(*libflv.VideoTag); okv {
			//				err = NewVideoMessage(MessageBase{
			//					rtmp:            rtmp,
			//					messageTime:     videoTag.GetTagInfo().TimeStamp,
			//					messageLength:   videoTag.GetTagInfo().DataSize,
			//					messageType:     MessageType(videoTag.GetTagInfo().TagType),
			//					messageStreamID: 0,
			//				},  videoTag).Send()
			//			}
			//			if err != nil {
			//				fmt.Println("send video data error:", err)
			//			}
			//		}
			//		rtmp.start = true
			//		return true
			//	}

			//	mb := MessageBase{
			//		rtmp:            rtmp,
			//		messageTime:     tag.GetTagInfo().TimeStamp,
			//		messageLength:   tag.GetTagInfo().DataSize,
			//		messageType:     MessageType(tag.GetTagInfo().TagType),
			//		messageStreamID: 0,
			//	}
			//	var err error
			//	if audioTag, oka := tag.(*libflv.AudioTag); oka {
			//		err = NewAudioMessage(mb, audioTag).Send()
			//	} else if videoTag, okv := tag.(*libflv.VideoTag); okv {
			//		err = NewVideoMessage(mb,videoTag).Send()
			//	}
			//	if err != nil {
			//		fmt.Println("send video data error:", err)
			//	}
			//}
			return true
		})
	}
}
