package librtmp

import (
	"fmt"
	"io"
	"net"

	"github.com/SmartBrave/Athena/easyio"
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
	role             connRole

	// Peer-advertised flow control state.
	peerWindowAckSize  uint32
	peerBandwidth      uint32
	peerBandwidthLimit LimitType

	// Inbound byte counter for Window Acknowledgement. Every `windowSize`
	// bytes we must reply with an Acknowledgement so the peer's send
	// window can slide forward (RTMP 1.0 §5.4.3).
	ownWindowAckSize uint32
	bytesReceived    uint32
	lastAcked        uint32
}

type connRole uint8

const (
	roleUnknown   connRole = iota
	rolePublisher          //the peer publishes to us
	rolePlayer             //the peer pulls from us
)

func NewRTMP(conn net.Conn, peer string, server *server) (rtmp *RTMP) {
	return &RTMP{
		readerConn:       easyio.NewEasyReader(conn),
		writerConn:       easyio.NewEasyWriter(conn),
		lastChunk:        make(map[uint32]*Chunk),
		peerMaxChunkSize: 128,
		ownMaxChunkSize:  128,
		peer:             peer,
		server:           server,
		ownWindowAckSize: 2500000,
	}
}

func (rtmp *RTMP) HandlerServer() {
	defer rtmp.cleanup()

	err := HandshakeServer(rtmp)
	if err != nil {
		fmt.Println("handshake error:", err)
		return
	}
	fmt.Println("handshake done...")

	for {
		err = ParseMessage(rtmp)
		if err == io.EOF {
			fmt.Println("disconnect")
			break
		}
		if err != nil {
			fmt.Println("ParseMessage error:", err)
			//Parse errors on a TCP stream are usually unrecoverable:
			//a framing desync leaves us unable to locate the next
			//chunk boundary. Bail out and let cleanup run.
			break
		}
	}
}

// cleanup releases any room state the connection owned. For publishers
// it closes the GOP broadcast so every subscriber (RTMP/FLV/HLS) wakes
// up with alive=false and exits its read loop; it also removes the
// room from the owning App. For players it's a no-op — their goroutine
// observes the closed broadcast on its own.
func (rtmp *RTMP) cleanup() {
	if rtmp.room == nil {
		return
	}
	if rtmp.role == rolePublisher {
		if app, ok := rtmp.server.apps[rtmp.app]; ok {
			app.Delete(rtmp.room.RoomID)
		}
		rtmp.room.Close()
	}
	rtmp.room = nil
}

func (rtmp *RTMP) HandlerClient() {
	//TODO
}
