package libmp4

// Sample is one access unit (frame) for the media segment. Duration is
// in the track's timescale; CompositionTimeOffset accommodates B-frames
// (PTS = DTS + offset). For all-intra streams CompositionTimeOffset = 0
// and IsKey is true on every sample.
type Sample struct {
	Duration              uint32
	Size                  uint32
	IsKey                 bool
	CompositionTimeOffset int32 //may be negative on bidirectional GOPs
	Data                  []byte
}

// MediaSegmentParams describes one fragment (moof + mdat). BaseDecodeTime
// is the cumulative DTS in track timescale up to (but not including)
// the first sample of this fragment.
type MediaSegmentParams struct {
	TrackID         uint32
	SequenceNumber  uint32
	BaseDecodeTime  uint64
	Samples         []Sample
}

// BuildMediaSegment serialises a CMAF media segment — moof followed by
// mdat — using the supplied samples. The tfhd default-base-is-moof
// flag is set per CMAF rules so trun byte offsets are relative to the
// start of moof.
func BuildMediaSegment(p MediaSegmentParams) []byte {
	if len(p.Samples) == 0 {
		return nil
	}

	mdatBody := []byte{}
	for _, s := range p.Samples {
		mdatBody = append(mdatBody, s.Data...)
	}
	mdat := Box{Type: FourCC("mdat"), Body: mdatBody}.Bytes()

	//Build moof first with a placeholder data_offset of 0, calculate
	//the real offset (= len(moof) + 8 bytes for mdat size+type), then
	//patch the trun's data_offset field. Cleaner than a 2-pass build.
	moof := buildMoof(p, 0)
	dataOffset := len(moof) + 8
	moof = buildMoof(p, int32(dataOffset))

	out := make([]byte, 0, len(moof)+len(mdat))
	out = append(out, moof...)
	out = append(out, mdat...)
	return out
}

func buildMoof(p MediaSegmentParams, dataOffset int32) []byte {
	return container("moof",
		mfhd(p.SequenceNumber),
		traf(p, dataOffset),
	)
}

func mfhd(seq uint32) []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, seq)
	return Box{Type: FourCC("mfhd"), Body: body}.Bytes()
}

func traf(p MediaSegmentParams, dataOffset int32) []byte {
	return container("traf",
		tfhd(p.TrackID),
		tfdt(p.BaseDecodeTime),
		trun(p.Samples, dataOffset),
	)
}

// tfhd uses default-base-is-moof (flag 0x020000) per CMAF: byte
// offsets in trun are computed from the moof box origin rather than
// any track-level base.
func tfhd(trackID uint32) []byte {
	const flagDefaultBaseIsMoof = 0x020000
	body := FullBoxHeader(0, flagDefaultBaseIsMoof)
	body = appendU32(body, trackID)
	return Box{Type: FourCC("tfhd"), Body: body}.Bytes()
}

func tfdt(baseTime uint64) []byte {
	body := FullBoxHeader(1, 0) //version 1 → 64-bit baseMediaDecodeTime
	body = appendU64(body, baseTime)
	return Box{Type: FourCC("tfdt"), Body: body}.Bytes()
}

// trun emits one entry per sample. Flags select which per-sample
// fields are present:
//   0x000001 data-offset present
//   0x000100 sample-duration present
//   0x000200 sample-size present
//   0x000400 sample-flags present
//   0x000800 sample-composition-time-offsets present
const (
	trunFlagDataOffset       = 0x000001
	trunFlagSampleDuration   = 0x000100
	trunFlagSampleSize       = 0x000200
	trunFlagSampleFlags      = 0x000400
	trunFlagSampleCTSOffsets = 0x000800
)

// avcSampleFlags returns the 32-bit sample_flags field for a given
// keyframe state. The most relevant bits are sample_depends_on (2 for
// I, 1 for non-I) and sample_is_non_sync_sample (0 for I, 1 otherwise).
func avcSampleFlags(isKey bool) uint32 {
	if isKey {
		//is_leading=0 depends_on=2(I, no other deps) is_depended_on=0
		//has_redundancy=0 padding=0 is_non_sync=0 degradation=0
		return 0x02000000
	}
	//depends_on=1(non-I) is_non_sync=1 (set bit 16)
	return 0x01010000
}

func trun(samples []Sample, dataOffset int32) []byte {
	flags := uint32(trunFlagDataOffset | trunFlagSampleDuration |
		trunFlagSampleSize | trunFlagSampleFlags |
		trunFlagSampleCTSOffsets)
	body := FullBoxHeader(1, flags) //version 1 so CTS offset is signed
	body = appendU32(body, uint32(len(samples)))
	body = appendU32(body, uint32(dataOffset))

	for _, s := range samples {
		body = appendU32(body, s.Duration)
		body = appendU32(body, s.Size)
		body = appendU32(body, avcSampleFlags(s.IsKey))
		body = appendU32(body, uint32(s.CompositionTimeOffset))
	}
	return Box{Type: FourCC("trun"), Body: body}.Bytes()
}

// styp is a per-segment "segment type" box: optional but encouraged
// for CMAF/DASH so a player tuning into the middle of a stream can
// recognise it as a media segment.
func BuildSegmentType() []byte {
	body := []byte{}
	body = append(body, []byte("msdh")...) //major brand
	body = appendU32(body, 0)              //minor version
	body = append(body, []byte("msdh")...)
	body = append(body, []byte("msix")...)
	return Box{Type: FourCC("styp"), Body: body}.Bytes()
}
