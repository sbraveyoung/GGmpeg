package librtsp

import (
	"encoding/binary"
)

// RTP fixed-header geometry (RFC 3550 §5.1). We never emit padding,
// extension headers, or CSRC entries, so the header is always 12 bytes.
const rtpHeaderSize = 12

// rtpVersion is the version field (2 bits, value 2 since RFC 3550).
const rtpVersion = 2

// RTPPacker constructs RTP packets for a single track. SequenceNumber
// is incremented per packet; the 32-bit timestamp is supplied by the
// caller (clock rate is per-payload — 90 kHz for H.264, sample-rate
// for AAC).
type RTPPacker struct {
	PayloadType uint8
	SSRC        uint32
	seq         uint16
}

// NewRTPPacker returns a packer with a randomised initial sequence
// number (the spec recommends randomising to defeat replay attacks).
// Tests that need determinism should set Seq directly afterwards.
func NewRTPPacker(payloadType uint8, ssrc uint32) *RTPPacker {
	//Random-ish initial seq from the SSRC; cheap and deterministic
	//enough for tests (set p.seq=0 to override).
	return &RTPPacker{
		PayloadType: payloadType,
		SSRC:        ssrc,
		seq:         uint16(ssrc & 0xFFFF),
	}
}

// Pack assembles one RTP packet. marker carries the M-bit which has
// payload-specific semantics (set on last fragment of a video frame,
// or on every audio frame).
func (p *RTPPacker) Pack(marker bool, timestamp uint32, payload []byte) []byte {
	out := make([]byte, rtpHeaderSize+len(payload))
	out[0] = byte(rtpVersion << 6) //V=2, P=0, X=0, CC=0
	out[1] = p.PayloadType & 0x7F
	if marker {
		out[1] |= 0x80
	}
	binary.BigEndian.PutUint16(out[2:], p.seq)
	binary.BigEndian.PutUint32(out[4:], timestamp)
	binary.BigEndian.PutUint32(out[8:], p.SSRC)
	copy(out[12:], payload)
	p.seq++
	return out
}

// Seq returns the sequence number that will appear on the next packet.
// Mainly useful for tests asserting wrap behaviour.
func (p *RTPPacker) Seq() uint16 { return p.seq }

// SetSeq overrides the sequence number — used by tests for deterministic
// output.
func (p *RTPPacker) SetSeq(s uint16) { p.seq = s }

// InterleaveFrame wraps a single RTP (or RTCP) packet in the RTSP
// interleaved-binary framing per RFC 2326 §10.12:
//
//	$<channel:1><length:2><RTP packet>
//
// channel is the lower of the two SETUP-negotiated values for the
// track (RTP gets the even channel, RTCP the odd one).
func InterleaveFrame(channel uint8, payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	out[0] = '$'
	out[1] = channel
	binary.BigEndian.PutUint16(out[2:4], uint16(len(payload)))
	copy(out[4:], payload)
	return out
}
