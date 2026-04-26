package librtsp

import (
	"encoding/binary"
	"fmt"
)

// RTPPacket is one parsed RTP packet's metadata + payload. We only
// surface the fields downstream depacketisers actually need; the rest
// (CSRC, padding, extension headers) are validated and skipped.
type RTPPacket struct {
	PayloadType uint8
	SequenceNum uint16
	Timestamp   uint32
	SSRC        uint32
	Marker      bool
	Payload     []byte
}

// ParseRTP returns the RTPPacket described by buf or an error if the
// header is malformed. The payload slice aliases buf so callers must
// copy if they intend to retain it.
func ParseRTP(buf []byte) (*RTPPacket, error) {
	if len(buf) < rtpHeaderSize {
		return nil, fmt.Errorf("RTP too short: %d bytes", len(buf))
	}
	v := buf[0] >> 6
	if v != rtpVersion {
		return nil, fmt.Errorf("unsupported RTP version %d", v)
	}
	padding := buf[0]&0x20 != 0
	extension := buf[0]&0x10 != 0
	cc := int(buf[0] & 0x0F)
	off := rtpHeaderSize + 4*cc
	if len(buf) < off {
		return nil, fmt.Errorf("RTP truncated at CSRC list")
	}
	if extension {
		if len(buf) < off+4 {
			return nil, fmt.Errorf("RTP truncated at extension header")
		}
		extLen := int(binary.BigEndian.Uint16(buf[off+2:off+4])) * 4
		off += 4 + extLen
		if len(buf) < off {
			return nil, fmt.Errorf("RTP truncated in extension")
		}
	}
	end := len(buf)
	if padding {
		if end-off < 1 {
			return nil, fmt.Errorf("RTP padding flag with no body")
		}
		padLen := int(buf[end-1])
		if padLen > end-off {
			return nil, fmt.Errorf("RTP padding %d > body %d", padLen, end-off)
		}
		end -= padLen
	}
	return &RTPPacket{
		PayloadType: buf[1] & 0x7F,
		Marker:      buf[1]&0x80 != 0,
		SequenceNum: binary.BigEndian.Uint16(buf[2:4]),
		Timestamp:   binary.BigEndian.Uint32(buf[4:8]),
		SSRC:        binary.BigEndian.Uint32(buf[8:12]),
		Payload:     buf[off:end],
	}, nil
}

// H264Reassembler accumulates H.264 NAL units across multiple RTP
// packets. Single-NAL packets emit immediately; FU-A fragmented NALs
// are buffered until the End-of-fragment bit fires; STAP-A aggregation
// packets are split into their constituent NALs. The M-bit on the RTP
// packet signals end-of-access-unit so callers can flush a complete
// AU into AVCC for forwarding.
type H264Reassembler struct {
	fuBuf     []byte //in-progress FU-A reassembly
	pending   [][]byte
	pendingTS uint32
}

// Push feeds one parsed RTP packet into the reassembler. Returns the
// list of NAL units that completed an access unit (the M-bit was set
// on this packet) plus the access-unit timestamp; otherwise returns
// nil to indicate "still buffering".
func (r *H264Reassembler) Push(pkt *RTPPacket) (nals [][]byte, ts uint32) {
	if len(pkt.Payload) == 0 {
		return nil, 0
	}
	r.pendingTS = pkt.Timestamp
	hdr := pkt.Payload[0]
	nalType := hdr & 0x1F

	switch {
	case nalType >= 1 && nalType <= 23:
		//Single NAL unit packet — payload is the NAL itself.
		r.pending = append(r.pending, append([]byte(nil), pkt.Payload...))
	case nalType == 24:
		//STAP-A: 1-byte header then a series of [size:2][NAL].
		body := pkt.Payload[1:]
		for len(body) >= 2 {
			n := int(binary.BigEndian.Uint16(body[:2]))
			body = body[2:]
			if n > len(body) {
				break
			}
			r.pending = append(r.pending, append([]byte(nil), body[:n]...))
			body = body[n:]
		}
	case nalType == 28:
		//FU-A: payload[0] is the FU indicator (NRI bits + type=28),
		//payload[1] is the FU header (S/E/R + original NAL type).
		if len(pkt.Payload) < 2 {
			return nil, 0
		}
		fuHeader := pkt.Payload[1]
		start := fuHeader&0x80 != 0
		end := fuHeader&0x40 != 0
		origType := fuHeader & 0x1F
		if start {
			//Reconstruct the original NAL header from the FU
			//indicator's NRI bits + the FU header's type.
			r.fuBuf = []byte{(hdr & 0xE0) | origType}
		}
		r.fuBuf = append(r.fuBuf, pkt.Payload[2:]...)
		if end {
			r.pending = append(r.pending, r.fuBuf)
			r.fuBuf = nil
		}
	default:
		//FU-B / MTAP / etc — not implemented. Skip.
	}

	if pkt.Marker && len(r.pending) > 0 {
		nals = r.pending
		ts = r.pendingTS
		r.pending = nil
		return
	}
	return nil, 0
}

// AACAUExtract decodes a single MPEG4-GENERIC RTP payload (AAC-hbr)
// into one or more raw AAC frames. The AU-headers section uses
// 16-bit AU-headers-length (in bits) followed by N × (13-bit AU-size +
// 3-bit AU-Index/IndexDelta).
func AACAUExtract(payload []byte) [][]byte {
	if len(payload) < 2 {
		return nil
	}
	hdrBits := int(binary.BigEndian.Uint16(payload[:2]))
	hdrBytes := (hdrBits + 7) / 8
	if 2+hdrBytes > len(payload) {
		return nil
	}
	headers := payload[2 : 2+hdrBytes]
	body := payload[2+hdrBytes:]

	//Each AU-header is 16 bits = 13-bit size + 3-bit index. Walk
	//them via a tiny bit cursor.
	var auSizes []int
	for off := 0; off+2 <= len(headers); off += 2 {
		size := int(binary.BigEndian.Uint16(headers[off:off+2])) >> 3
		auSizes = append(auSizes, size)
	}
	var out [][]byte
	for _, size := range auSizes {
		if size <= 0 || size > len(body) {
			break
		}
		out = append(out, append([]byte(nil), body[:size]...))
		body = body[size:]
	}
	return out
}

// AVCCFromNALs concatenates NAL units into the AVCC format FLV expects:
// a 4-byte big-endian length prefix followed by the NAL bytes, repeated
// per NAL.
func AVCCFromNALs(nals [][]byte) []byte {
	total := 0
	for _, n := range nals {
		total += 4 + len(n)
	}
	out := make([]byte, 0, total)
	for _, n := range nals {
		var sz [4]byte
		binary.BigEndian.PutUint32(sz[:], uint32(len(n)))
		out = append(out, sz[:]...)
		out = append(out, n...)
	}
	return out
}

// ContainsKeyframe reports whether any NAL in the slice has type 5 (IDR
// slice). Used to set the FLV VideoTag.FrameType correctly.
func ContainsKeyframe(nals [][]byte) bool {
	for _, n := range nals {
		if len(n) > 0 && (n[0]&0x1F) == 5 {
			return true
		}
	}
	return false
}
