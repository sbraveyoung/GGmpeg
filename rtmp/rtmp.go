package rtmp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"reflect"

	"github.com/gwuhaolin/livego/protocol/amf"
	"github.com/pkg/errors"
)

type RTMP struct {
	conn net.Conn
}

func NewRTMP(conn net.Conn) (rtmp *RTMP) {
	return &RTMP{
		conn:          conn,
		serverVersion: 3,
	}
}

func (rtmp *RTMP) Handler() {
	err := rtmp.HandShake()
	if err != nil {
		fmt.Println("handshake error:", err)
		return
	}

	for {
		err := rtmp.parseBasicHeader()
		if err != nil {
			fmt.Println("parseBasicHeader error:", err)
			break
		}
		err = rtmp.parseMessageHeader()
		if err != nil {
			fmt.Println("parseMessageHeader error:", err)
			break
		}
		err = rtmp.parseMessage()
		if err != nil {
			fmt.Println("parseMessage error:", err)
			break
		}
		break
	}
}

func (rtmp *RTMP) parseMessage() error {
	b, err := rtmp.readN(int(rtmp.clientMessageLength))
	if err != nil {
		return errors.Wrap(err, "rtmp.readN")
	}
	r := bytes.NewBuffer(b)
	amfDecoder := amf.NewDecoder()
	v := amf.Version(amf.AMF0)
	if rtmp.clientMessageType == 17 {
		v = amf.AMF3
	}
	var array []interface{}
	array, err = amfDecoder.DecodeBatch(r, v)
	if err != nil && err != io.EOF {
		return errors.Wrap(err, "amfDecoder.Decode")
	}
	for index, a := range array {
		fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	}
	return nil
}

func (rtmp *RTMP) readChunk() error {
	err := rtmp.parseBasicHeader()
	if err != nil {
		return errors.Wrap(err, "parse basic header")
	}
	//XXX
	return nil
}
