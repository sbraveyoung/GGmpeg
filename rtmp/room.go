package rtmp

import "sync"

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
	Publisher sync.Map //peer ip+port, rtmp conn
	Player    sync.Map //peer ip+port, rtmp conn
}

func NewRoom(roomID string) *Room {
	return &Room{
		RoomID:    roomID,
		Publisher: sync.Map{},
		Player:    sync.Map{},
	}
}
