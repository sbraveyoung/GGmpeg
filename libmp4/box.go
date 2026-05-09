// Package libmp4 emits the subset of ISO/IEC 14496-12 (and 14496-15
// for AVC-in-MP4) needed to produce CMAF-style fragmented MP4 streams:
// an init segment (ftyp + moov) plus a sequence of media segments
// (moof + mdat). Supports H.264 video and AAC-LC audio only.
//
// References:
//   - ISO/IEC 14496-12:2015 (ISO Base Media File Format)
//   - ISO/IEC 14496-15:2014 (carriage of NAL unit structured video)
//   - ISO/IEC 14496-3 (AAC AudioSpecificConfig)
//   - DASH-IF "DASH-IF Implementation Guidelines: Restricted Timed
//     Text Profile" appendices on segment formats
package libmp4

import (
	"encoding/binary"
)

// Box is the simplest expression of an ISO BMFF container: emit a
// 4-byte big-endian length, the four-character type code, and a body.
// Sub-boxes nest by appending child Box.Bytes() into a parent's body.
type Box struct {
	Type [4]byte
	Body []byte
}

// FourCC turns a 4-letter string into a four-character code. It panics
// if the string isn't exactly 4 ASCII bytes — the caller is supposed
// to use this with literals like FourCC("moof").
func FourCC(s string) [4]byte {
	if len(s) != 4 {
		panic("libmp4: FourCC requires exactly 4 bytes: " + s)
	}
	var fc [4]byte
	copy(fc[:], s)
	return fc
}

// Bytes serialises the box: 4 bytes size, 4 bytes type, body.
func (b Box) Bytes() []byte {
	size := uint32(8 + len(b.Body))
	out := make([]byte, 0, size)
	out = appendU32(out, size)
	out = append(out, b.Type[:]...)
	out = append(out, b.Body...)
	return out
}

// FullBoxHeader returns the 4-byte version+flags prefix used by many
// boxes (mvhd, tkhd, mdhd, hdlr, …).
func FullBoxHeader(version uint8, flags uint32) []byte {
	out := make([]byte, 4)
	out[0] = version
	out[1] = byte(flags >> 16)
	out[2] = byte(flags >> 8)
	out[3] = byte(flags)
	return out
}

// Helpers below append big-endian integers and ASCII strings to a
// byte slice in the style most appropriate for box-body construction.
func appendU8(b []byte, v uint8) []byte   { return append(b, v) }
func appendU16(b []byte, v uint16) []byte { return append(b, byte(v>>8), byte(v)) }
func appendU24(b []byte, v uint32) []byte {
	return append(b, byte(v>>16), byte(v>>8), byte(v))
}
func appendU32(b []byte, v uint32) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	return append(b, buf[:]...)
}
func appendU64(b []byte, v uint64) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	return append(b, buf[:]...)
}

// container builds a box whose body is the concatenation of the given
// child boxes. Equivalent to Box{Type, concat(children)}.Bytes().
func container(typ string, children ...[]byte) []byte {
	body := []byte{}
	for _, c := range children {
		body = append(body, c...)
	}
	return Box{Type: FourCC(typ), Body: body}.Bytes()
}
