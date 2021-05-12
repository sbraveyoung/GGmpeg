package rtmp

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
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

type HandshakeMode int

const (
	SIMPLE   HandshakeMode = 0
	COMPLEX1 HandshakeMode = 1
	COMPLEX2 HandshakeMode = 2
)

// type client struct{}
// func (c *client)Handshake(conn rtmpConn)(err error){
// }

var (
	FMSKey = []byte{
		0x47, 0x65, 0x6e, 0x75, 0x69, 0x6e, 0x65, 0x20,
		0x41, 0x64, 0x6f, 0x62, 0x65, 0x20, 0x46, 0x6c,
		0x61, 0x73, 0x68, 0x20, 0x4d, 0x65, 0x64, 0x69,
		0x61, 0x20, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
		0x20, 0x30, 0x30, 0x31, // Genuine Adobe Flash Media Server 001
		0xf0, 0xee, 0xc2, 0x4a, 0x80, 0x68, 0xbe, 0xe8,
		0x2e, 0x00, 0xd0, 0xd1, 0x02, 0x9e, 0x7e, 0x57,
		0x6e, 0xec, 0x5d, 0x2d, 0x29, 0x80, 0x6f, 0xab,
		0x93, 0xb8, 0xe6, 0x36, 0xcf, 0xeb, 0x31, 0xae,
	}
	FPkey = []byte{
		0x47, 0x65, 0x6E, 0x75, 0x69, 0x6E, 0x65, 0x20,
		0x41, 0x64, 0x6F, 0x62, 0x65, 0x20, 0x46, 0x6C,
		0x61, 0x73, 0x68, 0x20, 0x50, 0x6C, 0x61, 0x79,
		0x65, 0x72, 0x20, 0x30, 0x30, 0x31, // Genuine Adobe Flash Player 001
		0xF0, 0xEE, 0xC2, 0x4A, 0x80, 0x68, 0xBE, 0xE8,
		0x2E, 0x00, 0xD0, 0xD1, 0x02, 0x9E, 0x7E, 0x57,
		0x6E, 0xEC, 0x5D, 0x2D, 0x29, 0x80, 0x6F, 0xAB,
		0x93, 0xB8, 0xE6, 0x36, 0xCF, 0xEB, 0x31, 0xAE,
	}
)

type Server struct {
	handshakeMode HandshakeMode

	//c0 s0
	clientVersion uint8
	serverVersion uint8
	//simple handshake c1 s1
	clientTimeStamp uint32
	serverTimeStamp uint32
	clientZero      uint32
	serverZero      uint32
	//simple handshake c2 s2
	clientRandom []byte
	serverRandom []byte

	//complex handshake c1 s1
	clientDigest []byte //32byte
	serverDigest []byte //32byte
	clientKey    []byte //128byte
	serverKey    []byte //128byte
}

func NewServer() (server *Server) {
	return &Server{
		serverVersion: 3,
	}
}

func (s *Server) parseC0(c0 []byte) {
	s.clientVersion = uint8(c0[0])
}

func (s *Server) parseC1(c1 []byte) {
	//try complex handshake first

	//key-digest
	keyBufOffset := uint32(8)
	digestBufOffset := uint32(8 + 764)
	s.handshakeMode = COMPLEX1

	try := 0
complex:
	keyOffset := binary.BigEndian.Uint32(c1[keyBufOffset+760 : keyBufOffset+764])
	digestOffset := binary.BigEndian.Uint32(c1[digestBufOffset:digestBufOffset:4])

	s.clientKey = c1[keyBufOffset+keyOffset : keyBufOffset+keyOffset+128]
	s.clientDigest = c1[digestBufOffset+4+digestOffset : digestBufOffset+4+digestOffset+32]

	joined := append([]byte{}, c1[:digestBufOffset+4+digestOffset]...)
	joined = append(joined, c1[digestBufOffset+4+digestOffset+32:]...)

	mac := hmac.New(sha256.New, FPkey[:30])
	mac.Write(joined)
	newDigest := mac.Sum(nil)

	if bytes.Compare(newDigest, s.clientDigest) == 0 {
		fmt.Println("complex handshake success.")
		return
	} else {
		if try == 0 {
			digestBufOffset = uint32(8)
			keyBufOffset = uint32(8 + 764)
			s.handshakeMode = COMPLEX2
			goto complex
		} else {
			fmt.Println("complex handshake fail, using simple handshake")
			goto simple
		}
	}

simple:
	s.handshakeMode = SIMPLE
	s.clientTimeStamp = binary.BigEndian.Uint32(c1[:4])
	s.clientZero = binary.BigEndian.Uint32(c1[4:8])
	s.clientRandom = c1[8:]
}
func (s *Server) parseC2(c2 []byte) {
	//TODO
}

func (s *Server) makeS0() (s0 []byte) {
	b := bytes.NewBuffer(s0)
	binary.Write(b, binary.BigEndian, s.serverVersion)
	return b.Bytes()
}

func (s *Server) makeS1() (s1 []byte) {
	s.serverTimeStamp = uint32(time.Now().Unix())
	s.serverRandom = make([]byte, S1_LEN-8)
	_, _ = rand.Read(s.serverRandom)
	b := bytes.NewBuffer(s1)
	binary.Write(b, binary.BigEndian, s.serverTimeStamp)

	switch s.handshakeMode {
	case SIMPLE:
		_, _ = b.Write([]byte{0, 0, 0, 0})
		_, _ = b.Write(s.serverRandom)
		return b.Bytes()
	case COMPLEX1:
		_, _ = b.Write([]byte{0x04, 0x05, 0x00, 0x01})
	case COMPLEX2:
		_, _ = b.Write([]byte{0x04, 0x05, 0x00, 0x01})
	default:
	}
	return nil
}

func (s *Server) makeS2() (s2 []byte) {
	b := bytes.NewBuffer(s2)
	binary.Write(b, binary.BigEndian, s.clientTimeStamp)
	binary.Write(b, binary.BigEndian, s.clientZero)
	binary.Write(b, binary.BigEndian, s.clientRandom)
	return b.Bytes()
}

func (s *Server) Handshake(rtmp *RTMP) (err error) {
	c0c1c2 := [C0_LEN + C1_LEN + C2_LEN]byte{}
	c0 := c0c1c2[:C0_LEN]
	c1 := c0c1c2[C0_LEN : C0_LEN+C1_LEN]
	c2 := c0c1c2[C0_LEN+C1_LEN:]

	err = rtmp.conn.ReadFull(c0)
	if err != nil {
		return errors.Wrap(err, "read c0 from conn")
	}
	fmt.Printf("c0:%x\n", c0)
	s.parseC0(c0)
	if s.clientVersion != s.serverVersion {
		return errors.New("invalid client version")
	}

	err = rtmp.conn.ReadFull(c1)
	if err != nil {
		return errors.Wrap(err, "read c1 from conn")
	}
	fmt.Printf("c1:%x\n", c1)

	s0 := s.makeS0()
	fmt.Printf("s0:%x\n", s0)

	s.parseC1(c1)

	err = rtmp.conn.WriteFull(s0)
	if err != nil {
		return errors.Wrap(err, "write s0 to conn")
	}

	s1 := s.makeS1()
	fmt.Printf("s1:%x\n", s1)
	err = rtmp.conn.WriteFull(s1)
	if err != nil {
		return errors.Wrap(err, "write s1 to conn")
	}

	err = rtmp.conn.ReadFull(c2)
	if err != nil {
		return errors.Wrap(err, "read c2 from conn")
	}
	fmt.Printf("c2:%x\n", c2)
	s.parseC2(c2)

	s2 := s.makeS2()
	err = rtmp.conn.WriteFull(s2)
	if err != nil {
		return errors.Wrap(err, "write s2 to conn")
	}
	fmt.Printf("s2:%x\n", s2)
	return nil
}
