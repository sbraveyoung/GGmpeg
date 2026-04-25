package librtsp

// H.264 RTP payloadization per RFC 6184. We use the two simplest modes:
//
//   - Single NAL unit packet — when the NAL fits within mtu, just send
//     the NAL bytes as the RTP payload.
//   - FU-A fragmentation — when the NAL is too large, split into N
//     fragments. Each fragment carries an FU indicator (mimics the
//     original NAL header but with type=28) plus an FU header that
//     marks Start/End-of-fragment and repeats the original NAL type.
//
// We don't implement STAP-A (aggregation packets) or FU-B (with DON);
// they're rarely needed for live re-streaming and add complexity.
//
// The default MTU is set conservatively to 1400 bytes so the resulting
// IP packet stays under typical Ethernet 1500-byte path MTU even after
// RTSP-interleave (4 bytes), RTP header (12 bytes) and IP/TCP headers
// (~52 bytes worst case).
const DefaultMTU = 1400

// PackNAL takes a single H.264 NAL unit (no start code, no length
// prefix) and emits one or more RTP payload byte slices that fit the
// given MTU. Each payload is ready to feed into RTPPacker.Pack — the
// caller is responsible for setting the M-bit on the packet that
// carries the last NAL of an access unit.
func PackNAL(nal []byte, mtu int) [][]byte {
	if mtu <= 0 {
		mtu = DefaultMTU
	}
	if len(nal) == 0 {
		return nil
	}
	if len(nal) <= mtu {
		//Single NAL unit packet (RFC 6184 §5.6).
		return [][]byte{append([]byte(nil), nal...)}
	}

	//FU-A: header is 2 bytes (FU indicator + FU header), then payload
	//bytes from the original NAL minus its 1-byte NAL header.
	nalHeader := nal[0]
	nri := (nalHeader & 0x60) >> 5
	nalType := nalHeader & 0x1f
	body := nal[1:] //skip the original NAL header

	fragSize := mtu - 2 //room for FU indicator + FU header
	if fragSize <= 0 {
		return nil
	}
	var out [][]byte
	for i := 0; i < len(body); i += fragSize {
		end := i + fragSize
		if end > len(body) {
			end = len(body)
		}
		fuIndicator := (nri << 5) | 28 //type 28 = FU-A
		fuHeader := nalType
		if i == 0 {
			fuHeader |= 0x80 //S = start
		}
		if end == len(body) {
			fuHeader |= 0x40 //E = end
		}
		pkt := make([]byte, 2+(end-i))
		pkt[0] = fuIndicator
		pkt[1] = fuHeader
		copy(pkt[2:], body[i:end])
		out = append(out, pkt)
	}
	return out
}

// SplitAVCC walks an AVCC-formatted (4-byte length prefix) buffer and
// returns each NAL unit as a separate slice. RTMP delivers H.264 in
// AVCC; we re-fragment per NAL because the "access unit = one or more
// NALs" model is what RTP wants.
func SplitAVCC(buf []byte) [][]byte {
	var out [][]byte
	for off := 0; off+4 <= len(buf); {
		size := int(buf[off])<<24 | int(buf[off+1])<<16 | int(buf[off+2])<<8 | int(buf[off+3])
		off += 4
		if size <= 0 || off+size > len(buf) {
			return out
		}
		out = append(out, buf[off:off+size])
		off += size
	}
	return out
}
