package librtsp

// H.265 / HEVC RTP payloadization per RFC 7798. Same shapes as H.264
// but with a 2-byte NAL header and a slightly different fragmentation
// indicator (NAL type 49 = FU). We support:
//
//   - Single NAL unit packet (NAL fits in MTU)
//   - Aggregation packet AP (type 48) — emitted for parameter sets
//     when convenient; we don't generate APs from raw frames yet
//   - Fragmentation FU (type 49) — split a NAL across packets
//
// HEVC NAL header layout (16 bits big-endian):
//
//	bit 0      F  (forbidden_zero_bit)
//	bits 1-6   Type (6 bits)
//	bits 7-12  LayerId (6 bits)
//	bits 13-15 TID (3 bits)
//
// FU header layout (1 byte):
//
//	bit 0   S  (start of fragmented NAL)
//	bit 1   E  (end of fragmented NAL)
//	bits 2-7 FuType (= original NAL Type)

// PackHEVCNAL fragments one HEVC NAL into RTP payload byte slices.
func PackHEVCNAL(nal []byte, mtu int) [][]byte {
	if mtu <= 0 {
		mtu = DefaultMTU
	}
	if len(nal) < 2 {
		return nil
	}
	if len(nal) <= mtu {
		return [][]byte{append([]byte(nil), nal...)}
	}

	//Original 2-byte NAL header.
	hdr0 := nal[0]
	hdr1 := nal[1]
	nalType := (hdr0 >> 1) & 0x3F
	body := nal[2:]

	//FU NAL header: same layer/TID, but type field replaced with 49.
	fuHdr0 := (hdr0 & 0x81) | (49 << 1) //preserve F + LayerId msb, override Type
	fuHdr1 := hdr1                      //LayerId lsb + TID unchanged

	fragSize := mtu - 3 //2 bytes payload-header + 1 byte FU header
	if fragSize <= 0 {
		return nil
	}
	var out [][]byte
	for i := 0; i < len(body); i += fragSize {
		end := i + fragSize
		if end > len(body) {
			end = len(body)
		}
		fu := nalType
		if i == 0 {
			fu |= 0x80 //S
		}
		if end == len(body) {
			fu |= 0x40 //E
		}
		pkt := make([]byte, 3+(end-i))
		pkt[0] = fuHdr0
		pkt[1] = fuHdr1
		pkt[2] = fu
		copy(pkt[3:], body[i:end])
		out = append(out, pkt)
	}
	return out
}

// HEVCContainsKeyframe reports whether any NAL in the slice carries an
// IDR-class HEVC picture: types 16-23 (BLA/IDR/CRA) per H.265 §7.4.2.2.
func HEVCContainsKeyframe(nals [][]byte) bool {
	for _, n := range nals {
		if len(n) < 1 {
			continue
		}
		t := (n[0] >> 1) & 0x3F
		if t >= 16 && t <= 23 {
			return true
		}
	}
	return false
}
