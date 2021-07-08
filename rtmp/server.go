package rtmp

import (
	"fmt"
	"net"
	"sync"
)

type Server struct {
	Port string               //":1935"
	Apps map[string]*sync.Map //appName, roomID, *room
}

func NewServer(port string, apps ...string) (s *Server) {
	s = &Server{
		Port: port,
		Apps: make(map[string]*sync.Map, len(apps)),
	}
	for _, app := range apps {
		s.Apps[app] = &sync.Map{}
	}
	return s
}

func (s *Server) Handler() (err error) {
	listener, err := net.Listen("tcp", s.Port)
	if err != nil {
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener.Accept error:", err)
			continue
		}
		peer := conn.RemoteAddr().String()
		go NewRTMP(conn, peer, s).HandlerServer()
	}
}
