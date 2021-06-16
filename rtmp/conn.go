package rtmp

import (
	"io"
	"net"

	"github.com/pkg/errors"
)

type RTMPConn struct {
	net.Conn
}

func (conn RTMPConn) ReadFull(b []byte) (err error) {
	var n int
	n, err = io.ReadFull(conn, b)
	if err == io.EOF {
		return err
	}
	if err != nil {
		return errors.Wrap(err, "rtmp.conn.Read")
	}
	if n != len(b) {
		return errors.New("do not read enough data from conn")
	}
	return err
}

func (conn RTMPConn) ReadN(n uint32) (b []byte, err error) {
	b = make([]byte, n)
	err = conn.ReadFull(b)
	return b, err
}

func (conn RTMPConn) ReadAll() (b []byte, err error) {
	return io.ReadAll(conn)
}

func (conn RTMPConn) WriteFull(b []byte) error {
	n, err := conn.Write(b)
	if err != nil {
		return errors.Wrap(err, "conn.Write")
	}
	if n != len(b) {
		return errors.New("do not write enough data to conn")
	}
	return nil
}
