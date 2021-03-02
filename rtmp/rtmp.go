package rtmp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	C0_LEN = 1
	C1_LEN = 1536
	C2_LEN = 1536
	S0_LEN = 1
	S1_LEN = 1536
	S2_LEN = 1536
)

type RTMP struct {
	conn            net.Conn
	clientVersion   uint8
	serverVersion   uint8
	clientTimeStamp uint32
	serverTimeStamp uint32
	clientZero      uint32
	serverZero      uint32
	clientRandom    []byte
	serverRandom    []byte
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
	}

	for {
	}
}

func (rtmp *RTMP) readChunk() {
}

func (rtmp *RTMP) HandShake() error {
	c0c1c2 := [C0_LEN + C1_LEN + C2_LEN]byte{}
	c0 := c0c1c2[:C0_LEN]
	c1 := c0c1c2[C0_LEN : C0_LEN+C1_LEN]
	c2 := c0c1c2[C0_LEN+C1_LEN:]
	var n int

	err := rtmp.read(c0)
	if err != nil {
		return errors.Wrap(err, "read c0 from conn")
	}
	// fmt.Printf("c0:%x\n", c0)
	rtmp.parseC0(c0)
	if rtmp.clientVersion != rtmp.serverVersion {
		return errors.New("invalid client version")
	}

	err = rtmp.read(c1)
	if err != nil {
		return errors.Wrap(err, "read c1 from conn")
	}
	// fmt.Printf("c1:%x\n", c1)
	rtmp.parseC1(c1)

	s0 := rtmp.makeS0()
	// fmt.Printf("s0:%x\n", s0)
	n, err = rtmp.conn.Write(s0)
	if err != nil {
		return errors.Wrap(err, "write s0 to conn")
	}
	if n != S0_LEN {
		return errors.New("write no s0 to conn")
	}

	s1 := rtmp.makeS1()
	// fmt.Printf("s1:%x\n", s1)
	n, err = rtmp.conn.Write(s1)
	if err != nil {
		return errors.Wrap(err, "write s1 to conn")
	}
	if n != S1_LEN {
		return errors.New("write no s1 to conn")
	}

	err = rtmp.read(c2)
	if err != nil {
		return errors.Wrap(err, "read c2 from conn")
	}
	// fmt.Printf("c2:%x\n", c2)
	rtmp.parseC2(c2)

	s2 := rtmp.makeS2()
	n, err = rtmp.conn.Write(s2)
	if err != nil {
		return errors.Wrap(err, "write s2 to conn")
	}
	if n != S2_LEN {
		return errors.New("write no s2 to conn")
	}
	// fmt.Printf("s2:%x\n", s2)
	return nil
}

func (rtmp *RTMP) readN(n int) (b []byte, err error) {
	b = make([]byte, 0, n)
	err = rtmp.read(b)
	return b, err
}

func (rtmp *RTMP) read(b []byte) (err error) {
	var n int
	n, err = rtmp.conn.Read(b)
	if err != nil {
		return errors.Wrap(err, "rtmp.conn.Read")
	}
	if n != cap(b) {
		return errors.New("do not read enough data from conn")
	}
	return err
}

func (rtmp *RTMP) parseC0(c0 []byte) {
	rtmp.clientVersion = 3
	if len(c0) == C0_LEN {
		rtmp.clientVersion = uint8(c0[0])
	}
}

func (rtmp *RTMP) parseC1(c1 []byte) {
	if len(c1) == C1_LEN {
		rtmp.clientTimeStamp = binary.BigEndian.Uint32(c1[:4])
		rtmp.clientZero = binary.BigEndian.Uint32(c1[4:8])
		rtmp.clientRandom = c1[8:]
	}
}

func (rtmp *RTMP) parseC2(c2 []byte) {
	//TODO
}

func (rtmp *RTMP) makeS0() (s0 []byte) {
	b := bytes.NewBuffer(s0)
	binary.Write(b, binary.BigEndian, rtmp.serverVersion)
	return b.Bytes()
}

func (rtmp *RTMP) makeS1() (s1 []byte) {
	rtmp.serverTimeStamp = uint32(time.Now().Unix())
	rtmp.serverRandom = make([]byte, S1_LEN-8)
	_, _ = rand.Read(rtmp.serverRandom)
	b := bytes.NewBuffer(s1)
	binary.Write(b, binary.BigEndian, rtmp.serverTimeStamp)
	_, _ = b.Write([]byte{0, 0, 0, 0})
	_, _ = b.Write(rtmp.serverRandom)
	return b.Bytes()
}

func (rtmp *RTMP) makeS2() (s2 []byte) {
	b := bytes.NewBuffer(s2)
	binary.Write(b, binary.BigEndian, rtmp.clientTimeStamp)
	binary.Write(b, binary.BigEndian, rtmp.clientZero)
	binary.Write(b, binary.BigEndian, rtmp.clientRandom)
	return b.Bytes()
}
