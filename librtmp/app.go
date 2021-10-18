package librtmp

import (
	"sync"

	"github.com/SmartBrave/GGmpeg/libhls"
)

type App struct {
	appName string
	rooms   *sync.Map //roomID, *Room
	hlsMode libhls.HLS_MODE
	hls     *sync.Map //roomID,*HLS
}

func NewApp(appName string) *App {
	return &App{
		appName: appName,
		rooms:   &sync.Map{},
		hlsMode: libhls.NONE,
		hls:     &sync.Map{},
	}
}

func (app *App) Load(roomID string) *Room {
	room, ok := app.rooms.Load(roomID)
	if !ok {
		return nil
	}
	return room.(*Room)
}

func (app *App) Store(roomID string, room *Room) {
	app.rooms.Store(roomID, room)
}
