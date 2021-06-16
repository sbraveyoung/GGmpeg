package rtmp

import (
	"fmt"
	"net"
)

type Server struct {
	Port string //":1935"
}

func NewServer(port string, apps ...string) (s *Server) {
	s = &Server{
		Port: port,
	}
	InitRooms(apps...)
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
		go NewRTMP(conn, peer).HandlerServer()
	}
}
