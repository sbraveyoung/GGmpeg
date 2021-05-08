package rtmp

import (
	"io"
	"net"

	"github.com/pkg/errors"
)

type rtmpConn struct {
	net.Conn
}

func (conn rtmpConn) ReadFull(b []byte) (err error) {
	var n int
	n, err = io.ReadFull(conn, b)
	if err != nil {
		return errors.Wrap(err, "rtmp.conn.Read")
	}
	if n != len(b) {
		return errors.New("do not read enough data from conn")
	}
	return err
}

func (conn rtmpConn) ReadN(n int) (b []byte, err error) {
	b = make([]byte, n)
	err = conn.ReadFull(b)
	return b, err
}

func (conn rtmpConn) WriteFull(b []byte) error {
	n, err := conn.Write(b)
	if err != nil {
		return errors.Wrap(err, "conn.Write")
	}
	if n != len(b) {
		return errors.New("do not write enough data to conn")
	}
	return nil
}
