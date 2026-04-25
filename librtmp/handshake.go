package librtmp

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

// Adobe digest/key block geometry (C1/S1).
//
// Each of the two 764-byte blocks (digest + key) contains either
//   - 4-byte offset marker   + 32-byte digest   + 728 bytes of random
//     ("digest spread" = 728 = 764 - 4 - 32), or
//   - 128-byte key           + 632 bytes of random + 4-byte offset marker
//     ("key spread" = 632 = 764 - 128 - 4).
//
// The offset marker is interpreted as the sum of its 4 bytes modulo the
// spread, which places the digest (or key) somewhere in the remaining
// bytes.
const (
	handshakeBlockSize   = 764
	handshakeDigestSpread = 728 //764 - 4(marker) - 32(digest)
	handshakeDigestSize  = 32
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
	err = rtmp.readerConn.ReadFull(c0)
	if err != nil {
		return errors.Wrap(err, "read c0 from conn")
	}
	p.parseC0(c0)
	if p.clientVersion != p.serverVersion {
		return errors.New("invalid client version")
	}

	err = rtmp.readerConn.ReadFull(c1)
	if err != nil {
		return errors.Wrap(err, "read c1 from conn")
	}

	s0 := p.makeS0()

	p.parseC1(c1)

	err = rtmp.writerConn.WriteFull(s0)
	if err != nil {
		return errors.Wrap(err, "write s0 to conn")
	}

	s1 := p.makeS1()
	err = rtmp.writerConn.WriteFull(s1)
	if err != nil {
		return errors.Wrap(err, "write s1 to conn")
	}

	s2 := p.makeS2()
	err = rtmp.writerConn.WriteFull(s2)
	if err != nil {
		return errors.Wrap(err, "write s2 to conn")
	}

	err = rtmp.readerConn.ReadFull(c2)
	if err != nil {
		return errors.Wrap(err, "read c2 from conn")
	}
	if err := p.parseC2(c2); err != nil {
		//Log but don't fail: many players send a half-formed C2 (zeroed
		//echo fields) yet proceed with the session. Rejecting them
		//would be strictly spec-correct but breaks too much in the
		//wild — matches FFmpeg's leniency.
		fmt.Println("parseC2 warning:", err)
	}

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
	//Try complex handshake first. The digest block lives either at
	//offset 8 (scheme 2 / COMPLEX2) or 8+764 (scheme 1 / COMPLEX1). We
	//probe scheme 1 first, then fall back to scheme 2, then finally to
	//the simple (no-digest) form.

	digestBufOffset := 8 + handshakeBlockSize
	p.handshakeMode = COMPLEX1

	try := 0
complex:
	digestOffset := (int(c1[digestBufOffset]) + int(c1[digestBufOffset+1]) + int(c1[digestBufOffset+2]) + int(c1[digestBufOffset+3])) % handshakeDigestSpread

	p.clientDigest = c1[digestBufOffset+4+digestOffset : digestBufOffset+4+digestOffset+handshakeDigestSize]

	joined := append([]byte{}, c1[:digestBufOffset+4+digestOffset]...)
	joined = append(joined, c1[digestBufOffset+4+digestOffset+handshakeDigestSize:]...)

	mac := hmac.New(sha256.New, FPkey[:30])
	mac.Write(joined)
	newDigest := mac.Sum(nil)

	if bytes.Equal(newDigest, p.clientDigest) {
		return
	} else {
		if try == 0 {
			digestBufOffset = 8
			p.handshakeMode = COMPLEX2
			try++
			goto complex
		} else {
			goto simple
		}
	}

simple:
	p.handshakeMode = SIMPLE
	p.clientTimeStamp = binary.BigEndian.Uint32(c1[:4])
	p.clientZero = binary.BigEndian.Uint32(c1[4:8])
	p.clientRandom = c1[8:]
}

// parseC2 validates that the peer echoed back our server random. For
// the simple handshake C2 is a direct copy of S1's random block; for
// the complex handshake it's an HMAC-SHA256 tag over S2[:1504] keyed
// by HMAC(FPkey, serverDigest). Players are notoriously lax about C2,
// so we warn on mismatch rather than abort.
func (p *Peer) parseC2(c2 []byte) error {
	if len(c2) != C2_LEN {
		return fmt.Errorf("C2 length %d, want %d", len(c2), C2_LEN)
	}
	switch p.handshakeMode {
	case SIMPLE:
		//Spec §5.2.4: C2[8..] echoes S1's random. Some clients zero
		//this; only validate when serverRandom is populated.
		if len(p.serverRandom) >= len(c2[8:]) {
			if !bytes.Equal(c2[8:], p.serverRandom[:len(c2[8:])]) {
				return errors.New("simple C2 random mismatch")
			}
		}
	case COMPLEX1, COMPLEX2:
		if len(p.serverDigest) == 0 {
			return nil
		}
		mac := hmac.New(sha256.New, FPkey)
		mac.Write(p.serverDigest)
		tmpDigest := mac.Sum(nil)

		mac = hmac.New(sha256.New, tmpDigest)
		mac.Write(c2[:C2_LEN-handshakeDigestSize])
		expected := mac.Sum(nil)
		if !bytes.Equal(expected, c2[C2_LEN-handshakeDigestSize:]) {
			return errors.New("complex C2 digest mismatch")
		}
	}
	return nil
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
		digestBufOffset = 8 + handshakeBlockSize
		fallthrough
	case COMPLEX2:
		copy(s1[4:8], []byte{0x04, 0x05, 0x00, 0x01})

		digestOffset := (int(s1[digestBufOffset]) + int(s1[digestBufOffset+1]) + int(s1[digestBufOffset+2]) + int(s1[digestBufOffset+3])) % handshakeDigestSpread

		joined := append([]byte{}, s1[:digestBufOffset+4+digestOffset]...)
		joined = append(joined, s1[digestBufOffset+4+digestOffset+handshakeDigestSize:]...)

		mac := hmac.New(sha256.New, FMSKey[:36])
		mac.Write(joined)
		digest := mac.Sum(nil)
		copy(s1[digestBufOffset+4+digestOffset:digestBufOffset+4+digestOffset+handshakeDigestSize], digest)
		p.serverDigest = append([]byte{}, digest...)
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
		mac.Write(s2[:S2_LEN-handshakeDigestSize])
		s2Digest := mac.Sum(nil)
		copy(s2[S2_LEN-handshakeDigestSize:S2_LEN], s2Digest)
	default:
	}
	return
}
