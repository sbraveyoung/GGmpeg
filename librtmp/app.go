package librtmp

import (
	"sync"

	"github.com/SmartBrave/GGmpeg/libdash"
	"github.com/SmartBrave/GGmpeg/libhls"
)

type App struct {
	appName     string
	rooms       *sync.Map //roomID, *Room
	hlsMode     libhls.HLS_MODE
	hlsDir      string
	hls         *sync.Map //roomID, *libhls.HLS
	dashEnabled bool
	dashDir     string
	dash        *sync.Map //roomID, *libdash.DASH
}

func NewApp(appName string) *App {
	return &App{
		appName: appName,
		rooms:   &sync.Map{},
		hlsMode: libhls.NONE,
		hlsDir:  "./data",
		hls:     &sync.Map{},
		dashDir: "./data",
		dash:    &sync.Map{},
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

func (app *App) Delete(roomID string) {
	if h, ok := app.hls.LoadAndDelete(roomID); ok {
		if hls, ok := h.(*libhls.HLS); ok {
			hls.Stop()
		}
	}
	if d, ok := app.dash.LoadAndDelete(roomID); ok {
		if dash, ok := d.(*libdash.DASH); ok {
			dash.Stop()
		}
	}
	app.rooms.Delete(roomID)
}

func (app *App) LoadHLS(roomID string) *libhls.HLS {
	h, ok := app.hls.Load(roomID)
	if !ok {
		return nil
	}
	return h.(*libhls.HLS)
}

func (app *App) StoreHLS(roomID string, hls *libhls.HLS) {
	app.hls.Store(roomID, hls)
}

func (app *App) LoadDASH(roomID string) *libdash.DASH {
	d, ok := app.dash.Load(roomID)
	if !ok {
		return nil
	}
	return d.(*libdash.DASH)
}

func (app *App) StoreDASH(roomID string, dash *libdash.DASH) {
	app.dash.Store(roomID, dash)
}
