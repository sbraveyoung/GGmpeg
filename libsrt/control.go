package libsrt

import (
	"encoding/binary"
)

// ACK control packet body. Per draft-sharabayko-srt §3.2.4 the
// minimum body is the cumulative ack number; the seven extra fields
// (RTT, jitter, etc.) are optional. We always emit the full set with
// zero placeholders for the fields we don't measure — FFmpeg accepts
// short ACKs but other receivers occasionally reject them.
type ACKBody struct {
	LastAckedSeq    uint32 //sequence number AFTER the last in-order packet
	RTT             uint32 //microseconds — 0 placeholder
	RTTVariance     uint32
	AvailableBuffer uint32 //packets — 0 placeholder
	ReceivingRate   uint32 //packets/sec
	LinkCapacity    uint32 //packets/sec
	ReceivingBytes  uint32 //bytes/sec
}

// MarshalACK assembles a 28-byte ACK body. Header (TypeInfo = ACK
// number) is supplied by the caller via MarshalControlHeader.
func (a *ACKBody) Marshal() []byte {
	out := make([]byte, 28)
	binary.BigEndian.PutUint32(out[0:4], a.LastAckedSeq)
	binary.BigEndian.PutUint32(out[4:8], a.RTT)
	binary.BigEndian.PutUint32(out[8:12], a.RTTVariance)
	binary.BigEndian.PutUint32(out[12:16], a.AvailableBuffer)
	binary.BigEndian.PutUint32(out[16:20], a.ReceivingRate)
	binary.BigEndian.PutUint32(out[20:24], a.LinkCapacity)
	binary.BigEndian.PutUint32(out[24:28], a.ReceivingBytes)
	return out
}

// LossRange describes one loss interval. Single packets have From==To.
type LossRange struct {
	From, To uint32
}

// MarshalNAK encodes one NAK loss list. Single sequence numbers are
// written as one 32-bit word; ranges are written as two 32-bit words
// where the first has its top bit set per spec §3.2.5.
func MarshalNAK(losses []LossRange) []byte {
	out := make([]byte, 0, 4*len(losses)*2)
	for _, lr := range losses {
		if lr.From == lr.To {
			var w [4]byte
			binary.BigEndian.PutUint32(w[:], lr.From&0x7FFFFFFF)
			out = append(out, w[:]...)
			continue
		}
		var w1, w2 [4]byte
		binary.BigEndian.PutUint32(w1[:], lr.From|0x80000000)
		binary.BigEndian.PutUint32(w2[:], lr.To&0x7FFFFFFF)
		out = append(out, w1[:]...)
		out = append(out, w2[:]...)
	}
	return out
}

// ParseNAK is the inverse of MarshalNAK — useful for tests and
// (eventually) a sender-side retransmission queue.
func ParseNAK(body []byte) []LossRange {
	var out []LossRange
	for off := 0; off+4 <= len(body); off += 4 {
		w := binary.BigEndian.Uint32(body[off : off+4])
		if w&0x80000000 != 0 {
			//Range start; pair with the next 4 bytes.
			if off+8 > len(body) {
				return out
			}
			from := w & 0x7FFFFFFF
			to := binary.BigEndian.Uint32(body[off+4 : off+8])
			out = append(out, LossRange{From: from, To: to})
			off += 4
			continue
		}
		out = append(out, LossRange{From: w, To: w})
	}
	return out
}

// SeqDiff returns a − b under SRT's 31-bit modular sequence arithmetic.
// Positive values mean a is "ahead of" b. Used by receiver bookkeeping
// to detect new packets, retransmits, and loss.
func SeqDiff(a, b uint32) int32 {
	const half = uint32(1) << 30
	const mask = uint32(1)<<31 - 1
	d := (a - b) & mask
	if d > half {
		//d is "negative" in modular arithmetic — wrap into int32 range.
		return -int32(mask - d + 1)
	}
	return int32(d)
}
