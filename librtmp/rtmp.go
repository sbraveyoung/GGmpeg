package librtmp

import (
	"fmt"
	"io"
	"net"

	"github.com/SmartBrave/utils_sb/easyio"
	"github.com/SmartBrave/utils_sb/gop"
)

type RTMP struct {
	readerConn       easyio.EasyReader
	writerConn       easyio.EasyWriter
	lastChunk        map[uint32]*Chunk //csid
	peerMaxChunkSize uint32
	ownMaxChunkSize  int
	peer             string
	app              string
	room             *Room
	server           *server
	playType         string
	gopReader        *gop.GOPReader //only for player
}

func NewRTMP(conn net.Conn, peer string, server *server) (rtmp *RTMP) {
	return &RTMP{
		readerConn:       easyio.NewEasyReader(conn),
		writerConn:       easyio.NewEasyWriter(conn),
		lastChunk:        make(map[uint32]*Chunk),
		peerMaxChunkSize: 128,
		ownMaxChunkSize:  128,
		peer:             peer,
		server:           server,
	}
}

func (rtmp *RTMP) HandlerServer() {
	err := HandshakeServer(rtmp)
	if err != nil {
		fmt.Println("handshake error:", err)
		return
	}
	fmt.Println("handshake done...")

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
