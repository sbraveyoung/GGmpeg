package rtmp

import (
	"fmt"
	"io"
	"net"

	"github.com/SmartBrave/utils/easyio"
)

type RTMP struct {
	conn             easyio.EasyReadWriter
	lastChunk        map[uint32]*Chunk //csid
	peerMaxChunkSize uint32
	ownMaxChunkSize  int
	peer             string
	app              string
	room             *Room
}

func NewRTMP(conn net.Conn, peer string) (rtmp *RTMP) {
	return &RTMP{
		conn: RTMPConn{
			Conn: conn,
		},
		lastChunk:        make(map[uint32]*Chunk),
		peerMaxChunkSize: 128,
		ownMaxChunkSize:  128,
		peer:             peer,
	}
}

func (rtmp *RTMP) HandlerServer() {
	err := HandshakeServer(rtmp)
	if err != nil {
		fmt.Println("handshake error:", err)
		return
	}

	for {
		fmt.Println("-----------------------------------")
		err = ParseMessage(rtmp)
		if err == io.EOF {
			fmt.Println("disconnect")
			break
		}
		if err != nil {
			fmt.Println("ParseMessage error:", err)
			continue
		}
	}
}

func (rtmp *RTMP) HandlerClient() {
	//TODO
}
