package rtmp

import (
	"fmt"
	"net"

	"github.com/SmartBrave/utils/easyio"
)

type RTMP struct {
	conn      easyio.EasyReadWriter
	message   map[uint32]Message //message stream id
	chunkSize uint32
}

func NewRTMP(conn net.Conn) (rtmp *RTMP) {
	return &RTMP{
		conn: rtmpConn{
			Conn: conn,
		},
		chunkSize: 128,
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
		chunk, err := ParseChunk(rtmp)
		if err != nil {
			fmt.Println("NewChunk error:", err)
			continue
		}

		err = ParseMessage(rtmp, chunk)
		if err != nil {
			fmt.Println("ParseMessage error:", err)
			continue
		}
	}
}
