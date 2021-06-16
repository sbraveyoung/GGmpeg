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

var (
	FMSKey = []byte{
		0x47, 0x65, 0x6e, 0x75, 0x69, 0x6e, 0x65, 0x20,
		0x41, 0x64, 0x6f, 0x62, 0x65, 0x20, 0x46, 0x6c,
		0x61, 0x73, 0x68, 0x20, 0x4d, 0x65, 0x64, 0x69,
		0x61, 0x20, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
		0x20, 0x30, 0x30, 0x31, // Genuine Adobe Flash Media Peer 001
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

type Peer struct {
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

func HandshakeServer(rtmp *RTMP) (err error) {
	c0c1c2 := [C0_LEN + C1_LEN + C2_LEN]byte{}
	c0 := c0c1c2[:C0_LEN]
	c1 := c0c1c2[C0_LEN : C0_LEN+C1_LEN]
	c2 := c0c1c2[C0_LEN+C1_LEN:]

	p := &Peer{
		serverVersion: 3,
	}
	err = rtmp.conn.ReadFull(c0)
	if err != nil {
		return errors.Wrap(err, "read c0 from conn")
	}
	fmt.Printf("c0:%x\n", c0)
	p.parseC0(c0)
	if p.clientVersion != p.serverVersion {
		return errors.New("invalid client version")
	}

	err = rtmp.conn.ReadFull(c1)
	if err != nil {
		return errors.Wrap(err, "read c1 from conn")
	}
	fmt.Printf("c1:len:%d, data: %x\n", len(c1), c1)

	s0 := p.makeS0()
	fmt.Printf("s0:%x\n", s0)

	p.parseC1(c1)

	err = rtmp.conn.WriteFull(s0)
	if err != nil {
		return errors.Wrap(err, "write s0 to conn")
	}

	s1 := p.makeS1()
	fmt.Printf("s1:len:%d, data: %x\n", len(s1), s1)
	err = rtmp.conn.WriteFull(s1)
	if err != nil {
		return errors.Wrap(err, "write s1 to conn")
	}

	s2 := p.makeS2()
	err = rtmp.conn.WriteFull(s2)
	if err != nil {
		return errors.Wrap(err, "write s2 to conn")
	}
	fmt.Printf("s2:%x\n", s2)

	err = rtmp.conn.ReadFull(c2)
	if err != nil {
		return errors.Wrap(err, "read c2 from conn")
	}
	fmt.Printf("c2:%x\n", c2)
	p.parseC2(c2)

	return nil
}

func HandshakeClient(rtmp *RTMP) (err error) {
	//TODO
	return nil
}

func (p *Peer) parseC0(c0 []byte) {
	p.clientVersion = uint8(c0[0])
}

func (p *Peer) parseC1(c1 []byte) {
	//try complex handshake first

	//key-digest
	// keyBufOffset := 8
	digestBufOffset := 8 + 764
	p.handshakeMode = COMPLEX1

	try := 0
complex:
	// fmt.Println("keyBufOffset:", keyBufOffset)
	// fmt.Println("digestBufOffset:", digestBufOffset)

	// keyOffset := (int(c1[keyBufOffset+760]) + int(c1[keyBufOffset+761]) + int(c1[keyBufOffset+762]) + int(c1[keyBufOffset+763]))
	//XXX: what's mean about 728?
	digestOffset := (int(c1[digestBufOffset]) + int(c1[digestBufOffset+1]) + int(c1[digestBufOffset+2]) + int(c1[digestBufOffset+3])) % 728

	// fmt.Println("keyOffset:", keyOffset)
	// fmt.Println("digestOffset:", digestOffset)

	// p.clientKey = c1[keyBufOffset+keyOffset : keyBufOffset+keyOffset+128]
	p.clientDigest = c1[digestBufOffset+4+digestOffset : digestBufOffset+4+digestOffset+32]

	joined := append([]byte{}, c1[:digestBufOffset+4+digestOffset]...)
	joined = append(joined, c1[digestBufOffset+4+digestOffset+32:]...)

	// fmt.Printf("joined:%x\n", joined)
	// fmt.Printf("client key:%x\n", FPkey[:30])

	mac := hmac.New(sha256.New, FPkey[:30])
	mac.Write(joined)
	newDigest := mac.Sum(nil)

	// fmt.Printf("newDigest, len:%d, data:%x\n", len(newDigest), newDigest)
	// fmt.Printf("clientDigest, len:%d, data:%x\n", len(p.clientDigest), p.clientDigest)

	if bytes.Compare(newDigest, p.clientDigest) == 0 {
		fmt.Println("complex handshake success.")
		return
	} else {
		if try == 0 {
			fmt.Println("complex handshake mode 1 fail, try mode 2")
			digestBufOffset = 8
			// keyBufOffset = 8 + 764
			p.handshakeMode = COMPLEX2
			try++
			goto complex
		} else {
			fmt.Println("complex handshake fail, using simple handshake")
			goto simple
		}
	}

simple:
	p.handshakeMode = SIMPLE
	p.clientTimeStamp = binary.BigEndian.Uint32(c1[:4])
	p.clientZero = binary.BigEndian.Uint32(c1[4:8])
	p.clientRandom = c1[8:]
}
func (p *Peer) parseC2(c2 []byte) {
	//TODO
}

func (p *Peer) makeS0() (s0 []byte) {
	b := bytes.NewBuffer(s0)
	binary.Write(b, binary.BigEndian, p.serverVersion)
	return b.Bytes()
}

func (p *Peer) makeS1() (s1 []byte) {
	p.serverTimeStamp = uint32(time.Now().Unix())
	s1 = make([]byte, S1_LEN)
	_, _ = rand.Read(s1[8:])
	binary.BigEndian.PutUint32(s1[0:4], p.serverTimeStamp)

	digestBufOffset := 8
	switch p.handshakeMode {
	case SIMPLE:
		copy(s1[4:8], []byte{0x0, 0x0, 0x0, 0x0})
		p.serverRandom = s1[8:]
	case COMPLEX1:
		digestBufOffset = 8 + 764
		fallthrough
	case COMPLEX2:
		copy(s1[4:8], []byte{0x04, 0x05, 0x00, 0x01})

		digestOffset := (int(s1[digestBufOffset]) + int(s1[digestBufOffset+1]) + int(s1[digestBufOffset+2]) + int(s1[digestBufOffset+3])) % 728
		fmt.Println("digestOffset:", digestOffset)

		joined := append([]byte{}, s1[:digestBufOffset+4+digestOffset]...)
		joined = append(joined, s1[digestBufOffset+4+digestOffset+32:]...)
		// fmt.Printf("joined:%x\n", joined)

		mac := hmac.New(sha256.New, FMSKey[:36])
		mac.Write(joined)
		digest := mac.Sum(nil)
		// fmt.Printf("digest:%x\n", digest)
		copy(s1[digestBufOffset+4+digestOffset:digestBufOffset+4+digestOffset+32], digest)
	default:
	}
	return s1
}

func (p *Peer) makeS2() (s2 []byte) {
	switch p.handshakeMode {
	case SIMPLE:
		b := bytes.NewBuffer(s2)
		binary.Write(b, binary.BigEndian, p.clientTimeStamp)
		binary.Write(b, binary.BigEndian, p.clientZero)
		binary.Write(b, binary.BigEndian, p.clientRandom)
		return b.Bytes()
	case COMPLEX1, COMPLEX2:
		s2 = make([]byte, S2_LEN)
		_, _ = rand.Read(s2)

		mac := hmac.New(sha256.New, FMSKey)
		mac.Write(p.clientDigest)
		tmpDigest := mac.Sum(nil)

		mac = hmac.New(sha256.New, tmpDigest)
		mac.Write(s2[:S2_LEN-32])
		s2Digest := mac.Sum(nil)
		copy(s2[S2_LEN-32:S2_LEN], s2Digest)
	default:
	}
	return
}
