package libmp4

// HEVCInitParams describes one HEVC video track's init-segment metadata.
// HVCCRecord must be the publisher-provided HEVCDecoderConfigurationRecord
// (ISO/IEC 14496-15 §8.3.3.1.2) which is embedded verbatim into the
// hvcC sub-box — matches what FFmpeg does and avoids re-parsing the
// VPS/SPS/PPS into individual fields.
type HEVCInitParams struct {
	TrackID    uint32
	Timescale  uint32
	Width      uint16
	Height     uint16
	HVCCRecord []byte //full HEVCDecoderConfigurationRecord
}

// BuildHEVCInitSegment returns ftyp + moov for a single H.265 track.
// Mirrors BuildInitSegment but emits hev1 + hvcC instead of avc1 +
// avcC. Players that don't support HEVC quietly skip the file.
func BuildHEVCInitSegment(p HEVCInitParams) []byte {
	out := []byte{}
	out = append(out, ftyp()...)
	out = append(out, hevcMoov(p)...)
	return out
}

func hevcMoov(p HEVCInitParams) []byte {
	return container("moov",
		mvhd(p.Timescale),
		hevcTrak(p),
		mvex(p.TrackID),
	)
}

func hevcTrak(p HEVCInitParams) []byte {
	tk := InitSegmentParams{TrackID: p.TrackID, Width: p.Width, Height: p.Height}
	return container("trak", tkhd(tk), hevcMdia(p))
}

func hevcMdia(p HEVCInitParams) []byte {
	return container("mdia", mdhd(p.Timescale), hdlr("vide", "VideoHandler"), hevcMinf(p))
}

func hevcMinf(p HEVCInitParams) []byte {
	return container("minf", vmhd(), dinf(), hevcStbl(p))
}

func hevcStbl(p HEVCInitParams) []byte {
	return container("stbl",
		hevcStsd(p),
		emptyFullBox("stts"),
		emptyFullBox("stsc"),
		emptyStsz(),
		emptyFullBox("stco"),
	)
}

func hevcStsd(p HEVCInitParams) []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, 1) //entry_count
	body = append(body, hev1(p)...)
	return Box{Type: FourCC("stsd"), Body: body}.Bytes()
}

// hev1 is the H.265 SampleEntry. Shares the 78-byte VisualSampleEntry
// header layout with avc1; differs only in the 4-character box code
// and the embedded codec-config sub-box (hvcC vs avcC).
func hev1(p HEVCInitParams) []byte {
	body := []byte{}
	body = append(body, make([]byte, 6)...) //reserved
	body = appendU16(body, 1)               //data_reference_index

	body = append(body, make([]byte, 16)...) //pre_defined + reserved + pre_defined
	body = appendU16(body, p.Width)
	body = appendU16(body, p.Height)
	body = appendU32(body, 0x00480000) //horizres = 72 dpi
	body = appendU32(body, 0x00480000) //vertres
	body = appendU32(body, 0)          //reserved
	body = appendU16(body, 1)          //frame_count
	body = append(body, make([]byte, 32)...)
	body = appendU16(body, 0x0018) //depth (24)
	body = appendU16(body, 0xffff) //pre_defined = -1

	body = append(body, hvcC(p.HVCCRecord)...)
	return Box{Type: FourCC("hev1"), Body: body}.Bytes()
}

// hvcC wraps the publisher-supplied HEVCDecoderConfigurationRecord in
// an ISO BMFF box header. The body bytes pass through unchanged.
func hvcC(record []byte) []byte {
	return Box{Type: FourCC("hvcC"), Body: append([]byte(nil), record...)}.Bytes()
}
