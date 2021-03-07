package rtmp

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	// HANDSHAKE_MODE
	SIMPLE = iota
	COMPLEX
)

var (
	handshakeMode = SIMPLE
)

const (
	C0_LEN = 1
	C1_LEN = 1536
	C2_LEN = 1536
	S0_LEN = 1
	S1_LEN = 1536
	S2_LEN = 1536
)

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

const (
	FMT_0 = iota
	FMT_1
	FMT_2
	FMT_3
)

type RTMP struct {
	conn net.Conn

	//simple handshake
	clientVersion   uint8
	serverVersion   uint8
	clientTimeStamp uint32
	serverTimeStamp uint32
	clientZero      uint32
	serverZero      uint32

	clientRandom []byte
	serverRandom []byte

	//complex handshake
	clientDigest []byte //32byte
	serverDigest []byte //32byte
	clientKey    []byte //128byte
	serverKey    []byte //128byte

	fmt  uint8
	csid uint32

	clientMessageTimeStampDelta    uint32
	clientMessageExtendedTimeStamp uint32
	clientMessageLength            uint32
	clientMessageType              uint8
	clientMessageStreamID          uint32
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

	fmt.Println("conning")
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
		break
	}
}

func (rtmp *RTMP) parseMessageHeader() error {
	switch rtmp.fmt {
	case FMT_0:
		b11, err := rtmp.readN(11)
		if err != nil {
			return errors.Wrap(err, "read message header from conn")
		}
		rtmp.clientMessageTimeStampDelta = uint32(0x00)<<24 | uint32(b11[0])<<16 | uint32(b11[1])<<8 | uint32(b11[2])
		rtmp.clientMessageLength = uint32(0x00)<<24 | uint32(b11[3])<<16 | uint32(b11[4])<<8 | uint32(b11[5])
		rtmp.clientMessageType = b11[6]
		rtmp.clientMessageStreamID = binary.LittleEndian.Uint32(b11[7:])
	case FMT_1:
		b7, err := rtmp.readN(7)
		if err != nil {
			return errors.Wrap(err, "read message header from conn")
		}
		rtmp.clientMessageTimeStampDelta = uint32(0x00)<<24 | uint32(b7[0])<<16 | uint32(b7[1])<<8 | uint32(b7[2])
		rtmp.clientMessageLength = uint32(0x00)<<24 | uint32(b7[3])<<16 | uint32(b7[4])<<8 | uint32(b7[5])
		rtmp.clientMessageType = b7[6]
	case FMT_2:
		b3, err := rtmp.readN(3)
		if err != nil {
			return errors.Wrap(err, "read message header from conn")
		}
		rtmp.clientMessageTimeStampDelta = uint32(0x00)<<24 | uint32(b3[0])<<16 | uint32(b3[1])<<8 | uint32(b3[2])
	case FMT_3:
		//do nothing
	default:
		//do nothing
	}

	if rtmp.clientMessageTimeStampDelta == 0xffffff {
		b, err := rtmp.readN(4)
		if err != nil {
			return errors.Wrap(err, "read extended timestamp from conn")
		}
		rtmp.clientMessageExtendedTimeStamp = binary.BigEndian.Uint32(b)
	}

	fmt.Println("message timestamp delta:", rtmp.clientMessageTimeStampDelta)
	fmt.Println("message length:", rtmp.clientMessageLength)
	fmt.Println("message type id:", rtmp.clientMessageType)
	fmt.Println("message stream id:", rtmp.clientMessageStreamID)
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

func (rtmp *RTMP) parseBasicHeader() error {
	b, err := rtmp.readN(1)
	if err != nil {
		return errors.Wrap(err, "read chunk header from conn")
	}
	fmt.Printf("basic header:%x\n", b)
	rtmp.parseFmt(b[0])
	err = rtmp.parseCsID(b[0])
	if err != nil {
		return errors.Wrap(err, "parse csid")
	}
	return nil
}

func (rtmp *RTMP) parseCsID(b byte) error {
	b &= 0x3f
	switch b {
	case 0x0:
		b1, err := rtmp.readN(1)
		if err != nil {
			return errors.Wrap(err, "read basic header fron conn")
		}
		rtmp.csid = uint32(uint8(b1[0])) + 64
	case 0x1:
		b2, err := rtmp.readN(2)
		if err != nil {
			return errors.Wrap(err, "read basic header fron conn")
		}
		rtmp.csid = uint32(uint8(b2[0])) + uint32(uint8(b2[1]))*256 + 64
	case 0x2:
		//XXX
	default:
		rtmp.csid = uint32(uint8(b))
	}
	fmt.Println("csid:", rtmp.csid)
	return nil
}

func (rtmp *RTMP) parseFmt(b byte) {
	b &= 0xc0
	b >>= 6
	rtmp.fmt = uint8(b)
	fmt.Println("fmt:", rtmp.fmt)
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
	fmt.Printf("c0:%x\n", c0)
	rtmp.parseC0(c0)
	if rtmp.clientVersion != rtmp.serverVersion {
		return errors.New("invalid client version")
	}

	err = rtmp.read(c1)
	if err != nil {
		return errors.Wrap(err, "read c1 from conn")
	}
	fmt.Printf("c1:%x\n", c1)

	s0 := rtmp.makeS0()
	fmt.Printf("s0:%x\n", s0)

	if handshakeMode == SIMPLE {
		rtmp.parseC1(c1)

		n, err = rtmp.conn.Write(s0)
		if err != nil {
			return errors.Wrap(err, "write s0 to conn")
		}
		if n != S0_LEN {
			return errors.New("write no s0 to conn")
		}

		s1 := rtmp.makeS1()
		fmt.Printf("s1:%x\n", s1)
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
		fmt.Printf("c2:%x\n", c2)
		rtmp.parseC2(c2)

		s2 := rtmp.makeS2()
		n, err = rtmp.conn.Write(s2)
		if err != nil {
			return errors.Wrap(err, "write s2 to conn")
		}
		if n != S2_LEN {
			return errors.New("write no s2 to conn")
		}
		fmt.Printf("s2:%x\n", s2)
	} else {
		//complex handshake: https://blog.csdn.net/win_lin/article/details/13006803
		rtmp.parseC1Complex(c1)

		rtmp.makeS1()
	}

	return nil
}

func (rtmp *RTMP) readN(n int) (b []byte, err error) {
	b = make([]byte, n)
	err = rtmp.read(b)
	return b, err
}

func (rtmp *RTMP) read(b []byte) (err error) {
	var n int
	n, err = rtmp.conn.Read(b)
	if err != nil {
		return errors.Wrap(err, "rtmp.conn.Read")
	}
	if n != len(b) {
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

func (rtmp *RTMP) parseC1Complex(c1 []byte) {
	if len(c1) == C1_LEN {
		//digest-key
		digest := c1[:764]
		key := c1[764:]

		digestOffset := binary.BigEndian.Uint32(digest[:4])
		keyOffset := binary.BigEndian.Uint32(key[760:])

		rtmp.clientDigest = digest[4+digestOffset : 4+digestOffset+32]
		rtmp.clientKey = key[keyOffset : keyOffset+128]

		clientDigestRandomJoined := append(c1[:4+digestOffset], c1[4+digestOffset+32:]...)

		mac := hmac.New(sha256.New, FPKey)
		mac.Write()

		//TODO:key-digest
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

func (rtmp *RTMP) makeS1Complex() (s1 []byte) {
}

func (rtmp *RTMP) makeS2() (s2 []byte) {
	b := bytes.NewBuffer(s2)
	binary.Write(b, binary.BigEndian, rtmp.clientTimeStamp)
	binary.Write(b, binary.BigEndian, rtmp.clientZero)
	binary.Write(b, binary.BigEndian, rtmp.clientRandom)
	return b.Bytes()
}
