package librtmp

import "sync"

type App struct {
	appName string
	rooms   *sync.Map //roomID, *Room
}

func NewApp(appName string) *App {
	return &App{
		appName: appName,
		rooms:   &sync.Map{},
	}
}

func (app *App) Load(roomID string) *Room {
	room, _ := app.rooms.Load(roomID)
	return room.(*Room)
}

func (app *App) Store(roomID string, room *Room) {
	app.rooms.Store(roomID, room)
}
