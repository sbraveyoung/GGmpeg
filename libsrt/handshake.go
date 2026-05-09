package libsrt

import (
	"encoding/binary"
	"errors"
)

// Handshake body geometry per draft-sharabayko-srt §3.2.1. Caller
// (publisher) sends an INDUCTION first; we reply with the same body
// but a synchronisation cookie, then accept the CONCLUSION and reply
// with our chosen socket id.
const (
	HandshakeBodySize = 48 //bytes — fixed for v4 / v5 base body

	HSTypeDone       int32 = 0  //final ack
	HSTypeAgreement  int32 = 1  //v5 induction
	HSTypeConclusion int32 = -1
	HSTypeRejection  int32 = 1002
)

const (
	srtVersion4   = 4 //pre-1.2.0
	srtVersion5   = 5 //1.2.0+
)

// Handshake fields we actually care about. The remaining fields
// (peer IP, ext-flags) are passed through unchanged.
type Handshake struct {
	Version          uint32
	EncryptionField  uint16
	ExtensionField   uint16
	InitialSequence  uint32
	MaxPacketSize    uint32
	MaxFlowWindow    uint32
	HandshakeType    int32
	SrtSocketID      uint32
	SyncCookie       uint32
	PeerIP           [16]byte
}

// ParseHandshake decodes the 48-byte handshake body.
func ParseHandshake(b []byte) (*Handshake, error) {
	if len(b) < HandshakeBodySize {
		return nil, errors.New("handshake body too short")
	}
	h := &Handshake{
		Version:         binary.BigEndian.Uint32(b[:4]),
		EncryptionField: binary.BigEndian.Uint16(b[4:6]),
		ExtensionField:  binary.BigEndian.Uint16(b[6:8]),
		InitialSequence: binary.BigEndian.Uint32(b[8:12]),
		MaxPacketSize:   binary.BigEndian.Uint32(b[12:16]),
		MaxFlowWindow:   binary.BigEndian.Uint32(b[16:20]),
		HandshakeType:   int32(binary.BigEndian.Uint32(b[20:24])),
		SrtSocketID:     binary.BigEndian.Uint32(b[24:28]),
		SyncCookie:      binary.BigEndian.Uint32(b[28:32]),
	}
	copy(h.PeerIP[:], b[32:48])
	return h, nil
}

// Marshal renders the handshake body. Output length is always
// HandshakeBodySize.
func (h *Handshake) Marshal() []byte {
	out := make([]byte, HandshakeBodySize)
	binary.BigEndian.PutUint32(out[:4], h.Version)
	binary.BigEndian.PutUint16(out[4:6], h.EncryptionField)
	binary.BigEndian.PutUint16(out[6:8], h.ExtensionField)
	binary.BigEndian.PutUint32(out[8:12], h.InitialSequence)
	binary.BigEndian.PutUint32(out[12:16], h.MaxPacketSize)
	binary.BigEndian.PutUint32(out[16:20], h.MaxFlowWindow)
	binary.BigEndian.PutUint32(out[20:24], uint32(h.HandshakeType))
	binary.BigEndian.PutUint32(out[24:28], h.SrtSocketID)
	binary.BigEndian.PutUint32(out[28:32], h.SyncCookie)
	copy(out[32:48], h.PeerIP[:])
	return out
}

// IsInduction reports whether the handshake represents an Induction
// (v5) or version-4 hello. We accept either as the entry condition.
func (h *Handshake) IsInduction() bool {
	return h.HandshakeType == HSTypeAgreement || h.Version == srtVersion4
}
