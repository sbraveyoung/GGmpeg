package rtmp

import (
	"sync"

	"github.com/SmartBrave/GGmpeg/flv"
	"github.com/SmartBrave/utils/ring_buffer"
)

var (
	Apps = make(map[string]*sync.Map) //appName, roomID, *room
)

func InitRooms(apps ...string) {
	for _, app := range apps {
		Apps[app] = &sync.Map{}
	}
}

type Room struct {
	RoomID    string
	Publisher sync.Map //peer ip+port, rtmp conn. TODO: support multi publisher
	Player    sync.Map //peer ip+port, rtmp conn
	Meta      *DataMessage
	Cache     ring_buffer.Cache
}

func NewRoom(roomID string) *Room {
	r := &Room{
		RoomID:    roomID,
		Publisher: sync.Map{},
		Player:    sync.Map{},
		// Cache:     ring_buffer.NewRingBuffer(1024).Array().Build(),
		Cache: ring_buffer.NewRingBuffer(1024).Block().Build(),
	}
	go r.Transmit()
	return r
}

func (r *Room) Transmit() {
	for {
		tag, ok := r.Cache.Get().(flv.Tag)
		if !ok || tag == nil {
			continue
		}

		r.Player.Range(func(key, value interface{}) bool {
			// peer, _ := key.(string)
			rtmp, _ := value.(*RTMP)
			info := tag.GetTagInfo()
			mb := MessageBase{
				rtmp:             rtmp,
				messageTime:      info.TimeStamp,
				messageTimeDelta: 0,
				messageLength:    info.DataSize,
				messageType:      MessageType(info.TagType),
				messageStreamID:  0,
				messagePayload:   tag.Marshal(),
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
