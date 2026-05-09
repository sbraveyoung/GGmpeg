package libsrt

import (
	"encoding/binary"
)

// Tiny MPEG-TS demultiplexer. Just enough to split an SRT-delivered TS
// stream back into FLV-flavoured access units (H.264 NALs from the
// video PES, AAC ADTS frames from the audio PES). We don't validate
// CRC or chase DSCRPTRs — for known-good FFmpeg output the assumptions
// below hold, and a malformed packet just gets dropped.
//
// The flow is:
//
//	feed(TS bytes) →
//	  for each 188-byte packet:
//	    if PID == PAT → remember PMT PID
//	    if PID == PMT → remember audio/video PID + stream type
//	    if PID == video → accumulate into a PES buffer; on PUSI flush
//	      previous → split off NAL units → emit AccessUnit
//	    if PID == audio → same idea, emit Frame
//
// PES boundaries are detected via PUSI (payload unit start indicator).
// Each PES is delivered to the consumer as an AccessUnit / Frame
// callback so libsrt callers don't need to know about TS semantics.

const TSPacketSize = 188

// AccessUnit is one decoded video frame's worth of NAL units in
// AnnexB form (start-code prefixed). Timestamp is in 90 kHz units
// (matches the PTS field of the PES header).
type AccessUnit struct {
	PTS uint64
	DTS uint64
	Key bool   //starts with an IDR
	NAL []byte //AnnexB-formatted NAL bytes
}

// AudioFrame is one ADTS-prefixed AAC frame.
type AudioFrame struct {
	PTS  uint64
	Data []byte
}

// Demuxer accumulates TS packets and emits AccessUnit / AudioFrame.
// One Demuxer per session.
type Demuxer struct {
	pmtPID    uint16
	videoPID  uint16
	audioPID  uint16
	videoBuf  []byte
	videoTS   uint64 //PTS of in-progress PES (90 kHz)
	videoDTS  uint64
	audioBuf  []byte
	audioTS   uint64
	OnVideo   func(AccessUnit)
	OnAudio   func(AudioFrame)
}

// NewDemuxer returns a fresh demuxer. Callbacks should be set before
// calling Feed.
func NewDemuxer() *Demuxer { return &Demuxer{} }

// Feed processes one or more concatenated 188-byte TS packets. Bytes
// that don't make up a full packet are silently discarded; FFmpeg's
// SRT transport always packs complete TS packets per UDP datagram.
func (d *Demuxer) Feed(buf []byte) {
	for len(buf) >= TSPacketSize {
		pkt := buf[:TSPacketSize]
		buf = buf[TSPacketSize:]
		if pkt[0] != 0x47 {
			//Resync would be smart but rare in practice; bail out
			//on this datagram.
			return
		}
		pid := uint16(pkt[1]&0x1F)<<8 | uint16(pkt[2])
		pusi := pkt[1]&0x40 != 0
		afCtrl := (pkt[3] >> 4) & 0x03 //adaptation field control
		off := 4
		if afCtrl == 2 || afCtrl == 3 {
			afLen := int(pkt[4])
			off += 1 + afLen
			if off > TSPacketSize {
				continue
			}
		}
		if afCtrl == 0 || afCtrl == 2 {
			continue //no payload
		}
		payload := pkt[off:]

		switch pid {
		case 0x0000:
			d.parsePAT(payload)
		case d.pmtPID:
			d.parsePMT(payload)
		case d.videoPID:
			d.feedVideo(pusi, payload)
		case d.audioPID:
			d.feedAudio(pusi, payload)
		}
	}
}

// parsePAT pulls the first PMT entry from the Program Association
// Table. We only support a single program; multi-program TS would need
// a richer accumulator.
func (d *Demuxer) parsePAT(payload []byte) {
	if len(payload) < 2 {
		return
	}
	//Skip pointer + section header (8 bytes).
	pointer := int(payload[0])
	if 1+pointer+8 > len(payload) {
		return
	}
	body := payload[1+pointer:]
	//Section length covers everything from after section_length itself
	//up to and including the CRC. For our needs we just walk the
	//program loop right after the 8-byte fixed header.
	for off := 8; off+4 <= len(body)-4; off += 4 {
		programNumber := binary.BigEndian.Uint16(body[off : off+2])
		pid := binary.BigEndian.Uint16(body[off+2:off+4]) & 0x1FFF
		if programNumber != 0 {
			d.pmtPID = pid
			return
		}
	}
}

// parsePMT fishes the audio/video stream PIDs out of a Program Map
// Table. stream_type 0x1B = H.264, 0x24 = HEVC, 0x0F = AAC ADTS,
// 0x11 = LATM (we don't handle), 0xC1 = AC-3 (skipped).
func (d *Demuxer) parsePMT(payload []byte) {
	if len(payload) < 2 {
		return
	}
	pointer := int(payload[0])
	body := payload[1+pointer:]
	if len(body) < 12 {
		return
	}
	sectionLen := int(binary.BigEndian.Uint16(body[1:3])&0x0FFF) + 3
	if sectionLen > len(body) {
		sectionLen = len(body)
	}
	progInfoLen := int(binary.BigEndian.Uint16(body[10:12]) & 0x0FFF)
	off := 12 + progInfoLen
	for off+5 <= sectionLen-4 {
		streamType := body[off]
		streamPID := binary.BigEndian.Uint16(body[off+1:off+3]) & 0x1FFF
		esInfoLen := int(binary.BigEndian.Uint16(body[off+3:off+5]) & 0x0FFF)
		off += 5 + esInfoLen
		switch streamType {
		case 0x1B, 0x24:
			d.videoPID = streamPID
		case 0x0F:
			d.audioPID = streamPID
		}
	}
}

// feedVideo / feedAudio buffer PES bytes, flushing on PUSI boundaries.
// Internally we treat one PES unit as one access unit (true for live
// MPEG-TS produced by FFmpeg with -codec copy from H.264).
func (d *Demuxer) feedVideo(pusi bool, payload []byte) {
	if pusi && len(d.videoBuf) > 0 {
		d.flushVideo()
	}
	if pusi {
		pts, dts, body, ok := parsePESHeader(payload)
		if !ok {
			return
		}
		d.videoTS = pts
		d.videoDTS = dts
		d.videoBuf = append(d.videoBuf[:0], body...)
		return
	}
	d.videoBuf = append(d.videoBuf, payload...)
}

func (d *Demuxer) flushVideo() {
	if d.OnVideo == nil {
		d.videoBuf = d.videoBuf[:0]
		return
	}
	au := AccessUnit{
		PTS: d.videoTS,
		DTS: d.videoDTS,
		NAL: append([]byte(nil), d.videoBuf...),
		Key: containsAnnexBKeyframe(d.videoBuf),
	}
	d.videoBuf = d.videoBuf[:0]
	d.OnVideo(au)
}

func (d *Demuxer) feedAudio(pusi bool, payload []byte) {
	if pusi && len(d.audioBuf) > 0 {
		d.flushAudio()
	}
	if pusi {
		pts, _, body, ok := parsePESHeader(payload)
		if !ok {
			return
		}
		d.audioTS = pts
		d.audioBuf = append(d.audioBuf[:0], body...)
		return
	}
	d.audioBuf = append(d.audioBuf, payload...)
}

func (d *Demuxer) flushAudio() {
	if d.OnAudio == nil {
		d.audioBuf = d.audioBuf[:0]
		return
	}
	d.OnAudio(AudioFrame{
		PTS:  d.audioTS,
		Data: append([]byte(nil), d.audioBuf...),
	})
	d.audioBuf = d.audioBuf[:0]
}

// parsePESHeader pulls the PTS (and DTS, when present) out of a PES
// header and returns the residual elementary-stream body. The header
// is 9 bytes fixed + variable-length PES header data.
func parsePESHeader(b []byte) (pts, dts uint64, body []byte, ok bool) {
	if len(b) < 9 {
		return
	}
	if b[0] != 0x00 || b[1] != 0x00 || b[2] != 0x01 {
		return
	}
	flag := b[7] & 0xC0
	hdrLen := int(b[8])
	if 9+hdrLen > len(b) {
		return
	}
	switch flag {
	case 0x80:
		if hdrLen < 5 {
			return
		}
		pts = readPTS(b[9:14])
		dts = pts
	case 0xC0:
		if hdrLen < 10 {
			return
		}
		pts = readPTS(b[9:14])
		dts = readPTS(b[14:19])
	default:
		//No PTS — caller will see PTS==0.
	}
	body = b[9+hdrLen:]
	ok = true
	return
}

// readPTS decodes a 33-bit PTS/DTS field from a 5-byte timestamp.
func readPTS(b []byte) uint64 {
	if len(b) < 5 {
		return 0
	}
	v := uint64(b[0]&0x0E) << 29
	v |= uint64(b[1]) << 22
	v |= uint64(b[2]&0xFE) << 14
	v |= uint64(b[3]) << 7
	v |= uint64(b[4]&0xFE) >> 1
	return v
}

// containsAnnexBKeyframe detects a NAL header byte 0x65 (NAL type 5
// = IDR slice for H.264) anywhere after a start code.
func containsAnnexBKeyframe(buf []byte) bool {
	for i := 0; i+4 < len(buf); i++ {
		if buf[i] == 0 && buf[i+1] == 0 && buf[i+2] == 1 {
			if (buf[i+3] & 0x1F) == 5 {
				return true
			}
			i += 3
		} else if buf[i] == 0 && buf[i+1] == 0 && buf[i+2] == 0 && buf[i+3] == 1 {
			if (buf[i+4] & 0x1F) == 5 {
				return true
			}
			i += 4
		}
	}
	return false
}
