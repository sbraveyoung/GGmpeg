package rtmp

import (
	"fmt"
	"io"
	"net"

	"github.com/SmartBrave/utils/easyio"
)

type RTMP struct {
	conn      easyio.EasyReadWriter
	lastChunk *Chunk
	// message      map[uint32]Message //message stream id
	maxChunkSize int
}

func NewRTMP(conn net.Conn) (rtmp *RTMP) {
	return &RTMP{
		conn: rtmpConn{
			Conn: conn,
		},
		// message:      make(map[uint32]Message),
		maxChunkSize: 128,
	}
}

func (rtmp *RTMP) Handler() {
	err := NewServer().Handshake(rtmp)
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
