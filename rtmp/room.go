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
	ch        chan int
}

func NewRoom(roomID string) *Room {
	r := &Room{
		RoomID:  roomID,
		Players: sync.Map{},
		GOP:     make([]flv.Tag, 0, 1024),
		ch:      make(chan int, 1024),
	}
	go r.Transmit()
	return r
}

func (r *Room) Transmit() {
	for {
		<-r.ch
		// fmt.Println("333333333333333333333333333333")

		r.Players.Range(func(key, value interface{}) bool {
			// peer, _ := key.(string)
			rtmp, _ := value.(*RTMP)

			// tas := r.AudioSeq.(*flv.AudioTag)
			// tvs := r.VideoSeq.(*flv.VideoTag)

			// fmt.Printf("1111111111111111111111,tas.TagBase:%+v, soundFormat:%d, soundRate:%d, soundSize:%d, soundType:%d, AACPacketType:%d.  tvs.TagBase:%+v, FrameType:%d, CodecID:%d, AVCPacketType:%d, CompositionTime:%d. ", tas.TagBase, tas.SoundFormat, tas.SoundRate, tas.SoundSize, tas.SoundType, tas.AACPacketType, tvs.TagBase, tvs.FrameType, tvs.CodecID, tvs.AVCPacketType, tvs.CompositionTime)
			// if t, ok := tag.(*flv.AudioTag); ok {
			// fmt.Printf("tag is audio. t.TagBase:%+v, soundFormat:%d, soundRate:%d, soundSize:%d, soundType:%d, AACPacketType:%d\n", t.TagBase, t.SoundFormat, t.SoundRate, t.SoundSize, t.SoundType, t.AACPacketType)
			// } else if tv, ok := tag.(*flv.VideoTag); ok {
			// fmt.Printf("tag is video. t.TagBase:%+v, FrameType:%d, CodecID:%d, AVCPacketType:%d, CompositionTime:%d\n", tv.TagBase, tv.FrameType, tv.CodecID, tv.AVCPacketType, tv.CompositionTime)
			// } else {
			// fmt.Printf("invalid type----------------------------\n")
			// }
			if !rtmp.start {
				// fmt.Println("333333333333333333333333333333444444444444444444444444444")
				var err1, err2, err3 error
				err1 = NewDataMessage(MessageBase{
					rtmp: rtmp,
					// messageTime:      info.TimeStamp,
					messageTimeDelta: 0,
					// messageLength:    info.DataSize,
					messageType:     MessageType(DATA_MESSAGE_AMF0),
					messageStreamID: 0,
				}, r.Meta.FirstField, r.Meta.SecondField, r.Meta.MetaData).Send()
				err2 = NewVideoMessage(MessageBase{
					rtmp:             rtmp,
					messageTime:      r.VideoSeq.GetTagInfo().TimeStamp,
					messageTimeDelta: 0,
					messageLength:    r.VideoSeq.GetTagInfo().DataSize,
					messageType:      MessageType(r.VideoSeq.GetTagInfo().TagType),
					messageStreamID:  0,
				}, r.VideoSeq).Send()
				// err3 = NewVideoMessage(MessageBase{
				// rtmp:             rtmp,
				// messageTime:      r.AudioSeq.GetTagInfo().TimeStamp,
				// messageTimeDelta: 0,
				// messageLength:    r.AudioSeq.GetTagInfo().DataSize,
				// messageType:      MessageType(r.AudioSeq.GetTagInfo().TagType),
				// messageStreamID:  0,
				// }, r.AudioSeq).Send()
				err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
				if err != nil {
					fmt.Println("send meta and av seq error:", err)
				}
				for _, tag := range r.GOP {
					if _, oka := tag.(*flv.AudioTag); oka {
						//TODO
					} else if videoTag, okv := tag.(*flv.VideoTag); okv {
						err = NewVideoMessage(MessageBase{
							rtmp:             rtmp,
							messageTime:      videoTag.GetTagInfo().TimeStamp,
							messageTimeDelta: 0,
							messageLength:    videoTag.GetTagInfo().DataSize,
							messageType:      MessageType(videoTag.GetTagInfo().TagType),
							messageStreamID:  0,
						}, r.VideoSeq).Send()
						if err != nil {
							fmt.Println("send video data error:", err)
						}
					}
				}
				rtmp.start = true
				return true
			}

			// fmt.Println("3333333333333333333333333333335555555555555555555555555555")
			tag := r.GOP[len(r.GOP)-1]
			mb := MessageBase{
				rtmp:             rtmp,
				messageTime:      tag.GetTagInfo().TimeStamp,
				messageTimeDelta: 0,
				messageLength:    tag.GetTagInfo().DataSize,
				messageType:      MessageType(tag.GetTagInfo().TagType),
				messageStreamID:  0,
			}
			switch tag.GetTagInfo().TagType {
			case AUDIO_MESSAGE:
				NewAudioMessage(mb).Send()
			case VIDEO_MESSAGE:
				NewVideoMessage(mb).Send()
			default:
				//ignore
			}
			return true
		})
	}
}
