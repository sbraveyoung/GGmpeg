package rtmp

import (
	"fmt"
	"net"
	"sync"
)

type Server struct {
	Address string               //":1935"
	Apps    map[string]*sync.Map //appName, roomID, *room
}

func NewServer(address string, apps ...string) (s *Server) {
	s = &Server{
		Address: address,
		Apps:    make(map[string]*sync.Map, len(apps)),
	}
	for _, app := range apps {
		s.Apps[app] = &sync.Map{}
	}
	return s
}

func (s *Server) Handler() (err error) {
	var tcpAddr *net.TCPAddr
	var listener *net.TCPListener
	tcpAddr, err = net.ResolveTCPAddr("tcp", s.Address)
	if err != nil {
		return err
	}
	listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			fmt.Println("listener.AcceptTCP error:", err)
			continue
		}
		err = conn.SetNoDelay(true)
		if err != nil {
			fmt.Println("conn.SetNoDelay error:", err)
			continue
		}
		peer := conn.RemoteAddr().String()
		go NewRTMP(conn, peer, s).HandlerServer()
	}
}
