package rtmp

import (
	"fmt"
	"sync"
	"time"

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
	Cache     ring_buffer.Cache
}

func NewRoom(roomID string) *Room {
	m := &Room{
		RoomID:    roomID,
		Publisher: sync.Map{},
		Player:    sync.Map{},
		Cache:     ring_buffer.NewRingBuffer(1024).Array().Build(),
	}
	go m.Transmit()
	return m
}

func (m *Room) Transmit() {
	ticker := time.NewTicker(time.Millisecond * time.Duration(10))
	var tag *flv.Tag
	for {
		select {
		case <-ticker.C:
			tag, _ = m.Cache.Get().(*flv.Tag)
			if tag == nil {
				continue
			}
		}
		fmt.Println("get tag from buffer -----------------------------------------------------------------------")

		m.Player.Range(func(key, value interface{}) bool {
			// peer, _ := key.(string)
			rtmp, _ := value.(*RTMP)
			mb := MessageBase{
				rtmp:             rtmp,
				messageTime:      tag.TimeStamp,
				messageTimeDelta: 0,
				messageLength:    tag.DataSize,
				messageType:      MessageType(tag.TagType),
				messageStreamID:  0,
				messagePayload:   tag.Marshal(),
			}

			switch tag.TagType {
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
