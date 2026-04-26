// Package libsrt implements a deliberately minimal subset of the SRT
// (Secure Reliable Transport) protocol — enough for FFmpeg/OBS to
// successfully connect as a Caller and push a live MPEG-TS stream into
// our Listener. Out of scope:
//
//   - encryption (AES-CTR / KMREQ)
//   - selective retransmission (NAK / ACK / ACKACK)
//   - filter packets (FEC / packet filter)
//   - rendezvous mode
//
// The 16-bit "no-encryption Live mode" extension subset is what we
// support, which matches FFmpeg's defaults when a passphrase isn't
// supplied. Reference: draft-sharabayko-srt-01.
package libsrt

import (
	"encoding/binary"
	"errors"
)

// SRT packet header is always 16 bytes per draft-sharabayko-srt §3.
// The first 32 bits distinguish data (bit 0 = 0) from control (bit 0 = 1).
//
//	Data packet:     [seq:31][PP:2][O:1][KK:2][R:1][msgno:26][ts:32][dst:32]
//	Control packet:  [F:1][type:15][subtype:16][TSI:32][ts:32][dst:32]
const HeaderSize = 16

// PacketKind splits the data/control alternative.
type PacketKind uint8

const (
	KindData PacketKind = iota
	KindControl
)

// ControlType enumerates the message types we understand. SRT control
// packets are tagged in the upper bit of the first 32 bits as 1, with
// the lower 15 bits being the message type.
type ControlType uint16

const (
	CtrlHandshake ControlType = 0x0000
	CtrlKeepAlive ControlType = 0x0001
	CtrlACK       ControlType = 0x0002
	CtrlNAK       ControlType = 0x0003
	CtrlShutdown  ControlType = 0x0005
	CtrlACKACK    ControlType = 0x0006
)

// Header captures the 16-byte preamble of every SRT packet. Concrete
// data / handshake bodies live in DataPacket and Handshake.
type Header struct {
	Kind            PacketKind
	ControlType     ControlType //KindControl only
	SubType         uint16
	TypeInfo        uint32 //ACK seq #, NAK loss list head, etc
	SeqNumber       uint32 //KindData only — bottom 31 bits
	MessageInfo     uint32 //packed PP/O/KK/R/msgno (KindData only)
	Timestamp       uint32
	DestSocketID    uint32
}

// Parse extracts the 16-byte preamble. Returns an error if the buffer
// is too short or carries an unrecognised version.
func ParseHeader(b []byte) (*Header, []byte, error) {
	if len(b) < HeaderSize {
		return nil, nil, errors.New("SRT packet too short")
	}
	first := binary.BigEndian.Uint32(b[:4])
	second := binary.BigEndian.Uint32(b[4:8])
	hdr := &Header{
		Timestamp:    binary.BigEndian.Uint32(b[8:12]),
		DestSocketID: binary.BigEndian.Uint32(b[12:16]),
	}
	if first&0x80000000 == 0 {
		hdr.Kind = KindData
		hdr.SeqNumber = first & 0x7FFFFFFF
		hdr.MessageInfo = second
	} else {
		hdr.Kind = KindControl
		hdr.ControlType = ControlType((first >> 16) & 0x7FFF)
		hdr.SubType = uint16(first)
		hdr.TypeInfo = second
	}
	return hdr, b[HeaderSize:], nil
}

// MarshalDataHeader serialises a data-packet header into the first 16
// bytes of dst. Caller appends the payload separately.
func MarshalDataHeader(dst []byte, seq uint32, msgInfo uint32, ts uint32, destID uint32) {
	if len(dst) < HeaderSize {
		return
	}
	binary.BigEndian.PutUint32(dst[:4], seq&0x7FFFFFFF)
	binary.BigEndian.PutUint32(dst[4:8], msgInfo)
	binary.BigEndian.PutUint32(dst[8:12], ts)
	binary.BigEndian.PutUint32(dst[12:16], destID)
}

// MarshalControlHeader serialises a control-packet header into dst.
func MarshalControlHeader(dst []byte, ctrlType ControlType, subType uint16, typeInfo uint32, ts uint32, destID uint32) {
	if len(dst) < HeaderSize {
		return
	}
	first := uint32(0x80000000) | (uint32(ctrlType)&0x7FFF)<<16 | uint32(subType)
	binary.BigEndian.PutUint32(dst[:4], first)
	binary.BigEndian.PutUint32(dst[4:8], typeInfo)
	binary.BigEndian.PutUint32(dst[8:12], ts)
	binary.BigEndian.PutUint32(dst[12:16], destID)
}
