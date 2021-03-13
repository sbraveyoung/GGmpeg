package rtmp

import (
	"io"
	"net"

	"github.com/pkg/errors"
)

type rtmpConn struct {
	conn net.Conn
}

func (c *rtmpConn) read(b []byte) (err error) {
	var n int
	n, err = io.ReadFull(c.conn, b)
	if err != nil {
		return errors.Wrap(err, "rtmp.conn.Read")
	}
	if n != len(b) {
		return errors.New("do not read enough data from conn")
	}
	return err
}

func (c *rtmpConn) readN(n int) (b []byte, err error) {
	b = make([]byte, n)
	err = c.read(b)
	return b, err
}

func (c *rtmpConn) Write(b []byte) error {
	n, err := c.conn.Write(b)
	if err != nil {
		return errors.Wrap(err, "conn.Write")
	}
	if n != len(b) {
		return errors.New("do not write enough data to conn")
	}
	return nil
}
