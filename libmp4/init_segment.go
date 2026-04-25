package libmp4

// InitSegmentParams describes one video track's init-segment metadata.
// Audio is intentionally omitted from this minimal implementation —
// browsers like Shaka Player will play video-only DASH just fine, and
// supporting audio doubles the box-set this file emits.
type InitSegmentParams struct {
	TrackID  uint32 //must be > 0; conventionally 1
	Timescale uint32 //units per second (e.g. 90000 for video PTS)
	Width     uint16 //pixels
	Height    uint16
	SPS       []byte //one SPS NAL (without the 0x000001 start code)
	PPS       []byte //one PPS NAL (without the 0x000001 start code)
}

// BuildInitSegment returns a CMAF-compliant initialisation segment
// (ftyp + moov) for a single H.264 video track. The track's sample
// description box (avc1) embeds an avcC configuration record built
// from the supplied SPS/PPS — without that the player can't begin
// decoding. Caller must hand-fill TrackID / Timescale / dimensions
// to match the publisher's actual stream metadata.
func BuildInitSegment(p InitSegmentParams) []byte {
	out := []byte{}
	out = append(out, ftyp()...)
	out = append(out, moov(p)...)
	return out
}

// ftyp identifies this as a CMAF track file. Major brand "iso6" and
// compatibility brands "iso6 mp41 cmfc" cover Shaka Player, dash.js,
// hls.js, native macOS/iOS Safari (the last via fMP4-HLS).
func ftyp() []byte {
	body := []byte{}
	body = append(body, []byte("iso6")...) //major brand
	body = appendU32(body, 1)              //minor version
	body = append(body, []byte("iso6")...) //compatible
	body = append(body, []byte("mp41")...)
	body = append(body, []byte("cmfc")...)
	return Box{Type: FourCC("ftyp"), Body: body}.Bytes()
}

// moov is the top-level header for the init segment.
func moov(p InitSegmentParams) []byte {
	return container("moov",
		mvhd(p.Timescale),
		trak(p),
		mvex(p.TrackID),
	)
}

// mvhd carries movie-wide defaults. We use version 1 so timestamp /
// duration fields are 64 bits — useful when timescale is 90000 and
// stream length isn't known up front.
func mvhd(timescale uint32) []byte {
	body := FullBoxHeader(1, 0)
	body = appendU64(body, 0)               //creation_time
	body = appendU64(body, 0)               //modification_time
	body = appendU32(body, timescale)       //timescale
	body = appendU64(body, 0)               //duration (0 = unknown / live)
	body = appendU32(body, 0x00010000)      //rate (1.0)
	body = appendU16(body, 0x0100)          //volume (1.0)
	body = appendU16(body, 0)               //reserved
	body = append(body, make([]byte, 8)...) //reserved (2 × uint32)
	//Unity matrix.
	for _, v := range [9]uint32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000} {
		body = appendU32(body, v)
	}
	body = append(body, make([]byte, 24)...) //pre_defined (6 × uint32)
	body = appendU32(body, 2)                //next_track_ID
	return Box{Type: FourCC("mvhd"), Body: body}.Bytes()
}

func trak(p InitSegmentParams) []byte {
	return container("trak", tkhd(p), mdia(p))
}

// tkhd flags 0x07 = enabled | in-movie | in-preview.
func tkhd(p InitSegmentParams) []byte {
	body := FullBoxHeader(1, 0x000007)
	body = appendU64(body, 0)         //creation_time
	body = appendU64(body, 0)         //modification_time
	body = appendU32(body, p.TrackID) //track_ID
	body = appendU32(body, 0)         //reserved
	body = appendU64(body, 0)         //duration
	body = append(body, make([]byte, 8)...)
	body = appendU16(body, 0) //layer
	body = appendU16(body, 0) //alternate_group
	body = appendU16(body, 0) //volume (audio only)
	body = appendU16(body, 0) //reserved
	for _, v := range [9]uint32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000} {
		body = appendU32(body, v)
	}
	body = appendU32(body, uint32(p.Width)<<16)  //width as 16.16
	body = appendU32(body, uint32(p.Height)<<16) //height as 16.16
	return Box{Type: FourCC("tkhd"), Body: body}.Bytes()
}

func mdia(p InitSegmentParams) []byte {
	return container("mdia", mdhd(p.Timescale), hdlr("vide", "VideoHandler"), minf(p))
}

func mdhd(timescale uint32) []byte {
	body := FullBoxHeader(1, 0)
	body = appendU64(body, 0)
	body = appendU64(body, 0)
	body = appendU32(body, timescale)
	body = appendU64(body, 0)         //duration
	body = appendU16(body, 0x55c4)    //language code 'und' (5*32+0x800=0x55c4)
	body = appendU16(body, 0)         //pre_defined
	return Box{Type: FourCC("mdhd"), Body: body}.Bytes()
}

func hdlr(handlerType, name string) []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, 0) //pre_defined
	body = append(body, []byte(handlerType)...)
	body = append(body, make([]byte, 12)...) //reserved (3 × uint32)
	body = append(body, []byte(name)...)
	body = append(body, 0) //null terminator
	return Box{Type: FourCC("hdlr"), Body: body}.Bytes()
}

func minf(p InitSegmentParams) []byte {
	return container("minf", vmhd(), dinf(), stbl(p))
}

func vmhd() []byte {
	body := FullBoxHeader(0, 1) //flags=1 required by spec
	body = appendU16(body, 0)   //graphicsmode
	body = appendU16(body, 0)   //opcolor R
	body = appendU16(body, 0)
	body = appendU16(body, 0)
	return Box{Type: FourCC("vmhd"), Body: body}.Bytes()
}

func dinf() []byte {
	return container("dinf", dref())
}

func dref() []byte {
	url := FullBoxHeader(0, 1) //flags=1 → media is in same file
	urlBox := Box{Type: FourCC("url "), Body: url}.Bytes()
	body := FullBoxHeader(0, 0)
	body = appendU32(body, 1) //entry_count
	body = append(body, urlBox...)
	return Box{Type: FourCC("dref"), Body: body}.Bytes()
}

func stbl(p InitSegmentParams) []byte {
	//For fragmented MP4 the stsd describes the codec; the empty
	//stts/stsc/stsz/stco lists are required by spec but contain
	//zero entries because the actual sample tables live in moof/trun.
	return container("stbl",
		stsd(p),
		emptyFullBox("stts"),
		emptyFullBox("stsc"),
		emptyStsz(),
		emptyFullBox("stco"),
	)
}

func emptyFullBox(typ string) []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, 0) //entry_count
	return Box{Type: FourCC(typ), Body: body}.Bytes()
}

func emptyStsz() []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, 0) //sample_size (0 => per-sample sizes)
	body = appendU32(body, 0) //sample_count
	return Box{Type: FourCC("stsz"), Body: body}.Bytes()
}

func stsd(p InitSegmentParams) []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, 1) //entry_count
	body = append(body, avc1(p)...)
	return Box{Type: FourCC("stsd"), Body: body}.Bytes()
}

// avc1 is the SampleEntry for H.264. It contains a fixed 78-byte
// VisualSampleEntry header followed by an avcC sub-box describing the
// SPS/PPS.
func avc1(p InitSegmentParams) []byte {
	body := []byte{}
	body = append(body, make([]byte, 6)...) //reserved
	body = appendU16(body, 1)               //data_reference_index

	body = append(body, make([]byte, 16)...) //pre_defined + reserved + pre_defined
	body = appendU16(body, p.Width)
	body = appendU16(body, p.Height)
	body = appendU32(body, 0x00480000) //horizresolution = 72 dpi
	body = appendU32(body, 0x00480000) //vertresolution
	body = appendU32(body, 0)          //reserved
	body = appendU16(body, 1)          //frame_count
	body = append(body, make([]byte, 32)...)        //compressorname (32 bytes, all zero)
	body = appendU16(body, 0x0018)                  //depth (24)
	body = appendU16(body, 0xffff)                  //pre_defined = -1

	body = append(body, avcC(p.SPS, p.PPS)...)
	return Box{Type: FourCC("avc1"), Body: body}.Bytes()
}

// avcC packs an AVCDecoderConfigurationRecord per ISO 14496-15 §5.2.4.
func avcC(sps, pps []byte) []byte {
	if len(sps) < 4 {
		return Box{Type: FourCC("avcC")}.Bytes()
	}
	body := []byte{}
	body = appendU8(body, 1)         //configurationVersion
	body = appendU8(body, sps[1])    //AVCProfileIndication (from SPS byte 1)
	body = appendU8(body, sps[2])    //profile_compatibility (SPS byte 2)
	body = appendU8(body, sps[3])    //AVCLevelIndication (SPS byte 3)
	body = appendU8(body, 0xff)      //reserved(6) | lengthSizeMinusOne(2) = 3
	body = appendU8(body, 0xe1)      //reserved(3) | numOfSequenceParameterSets(5) = 1
	body = appendU16(body, uint16(len(sps)))
	body = append(body, sps...)
	body = appendU8(body, 1) //numOfPictureParameterSets
	body = appendU16(body, uint16(len(pps)))
	body = append(body, pps...)
	return Box{Type: FourCC("avcC"), Body: body}.Bytes()
}

func mvex(trackID uint32) []byte {
	return container("mvex", trex(trackID))
}

// trex carries default sample values referenced by per-fragment trun.
// We set defaults to 0 and let trun supply per-sample values; this
// matches what FFmpeg writes for fragmented MP4.
func trex(trackID uint32) []byte {
	body := FullBoxHeader(0, 0)
	body = appendU32(body, trackID)
	body = appendU32(body, 1) //default_sample_description_index
	body = appendU32(body, 0) //default_sample_duration
	body = appendU32(body, 0) //default_sample_size
	body = appendU32(body, 0) //default_sample_flags
	return Box{Type: FourCC("trex"), Body: body}.Bytes()
}
