package librtsp

import "encoding/binary"

// AAC over RTP per RFC 3640, mode AAC-hbr. We always emit one access
// unit per RTP packet (no fragmentation, no aggregation), which keeps
// the AU-Header section to a fixed 4 bytes:
//
//	AU-headers-length: 16 bits = 16 (one 16-bit AU-header)
//	AU-header:         13 bits AU-size + 3 bits AU-Index = 0
//
// followed by the raw AAC frame bytes (without ADTS — players parse
// the size from the AU-header and the codec parameters from the SDP
// fmtp config field).
func PackAACFrame(frame []byte) []byte {
	if len(frame) == 0 {
		return nil
	}
	if len(frame) >= 1<<13 {
		//13-bit AU-size limit. Live AAC frames are ≤ ~1.5 KB so this
		//ceiling is practically never hit; fail-soft by truncating.
		frame = frame[:(1<<13)-1]
	}
	out := make([]byte, 4+len(frame))
	//AU-headers-length: 16 bits in network order.
	binary.BigEndian.PutUint16(out[:2], 16)
	//AU-header: 13-bit AU-size shifted into the high 13 bits of a
	//16-bit word; AU-Index occupies the low 3 bits and is 0 for the
	//first AU in a packet.
	binary.BigEndian.PutUint16(out[2:4], uint16(len(frame))<<3)
	copy(out[4:], frame)
	return out
}

// AdtsToRaw strips the 7-byte ADTS header off an AAC frame. The HLS
// transcoder happens to splice ADTS in front of every AAC frame from
// libaac.AACHeader.Adts; for RTP we want the raw access unit instead.
func AdtsToRaw(frame []byte) []byte {
	if len(frame) < 7 {
		return frame
	}
	if frame[0] == 0xFF && (frame[1]&0xF0) == 0xF0 {
		return frame[7:]
	}
	return frame
}
