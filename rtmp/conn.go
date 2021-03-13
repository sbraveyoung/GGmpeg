package rtmp

import (
	"net"

	"github.com/pkg/errors"
)

type rtmpConn struct {
	conn net.Conn
}

func (c *rtmpConn) read(b []byte) (err error) {
	var n int
	n, err = c.conn.Read(b)
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
