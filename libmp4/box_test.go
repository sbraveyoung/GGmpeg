package libmp4

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// readBox is a tiny ISO BMFF parser used by tests to walk the output
// of BuildInitSegment / BuildMediaSegment without pulling in a real
// MP4 library. Returns the box's type, total size, and body bytes.
func readBox(t *testing.T, b []byte, off int) (typ string, size uint32, body []byte) {
	t.Helper()
	if off+8 > len(b) {
		t.Fatalf("readBox at %d: only %d bytes available", off, len(b))
	}
	size = binary.BigEndian.Uint32(b[off:])
	if int(size) < 8 || off+int(size) > len(b) {
		t.Fatalf("readBox at %d: bad size %d (have %d remaining)", off, size, len(b)-off)
	}
	typ = string(b[off+4 : off+8])
	body = b[off+8 : off+int(size)]
	return
}

// findBox searches the immediate children of a container body for a
// box of the given type and returns its body. Useful for asserting
// nested structure.
func findBox(t *testing.T, body []byte, typ string) []byte {
	t.Helper()
	for off := 0; off+8 <= len(body); {
		size := binary.BigEndian.Uint32(body[off:])
		if size < 8 {
			t.Fatalf("findBox: invalid size %d at %d", size, off)
		}
		if string(body[off+4:off+8]) == typ {
			return body[off+8 : off+int(size)]
		}
		off += int(size)
	}
	return nil
}

func TestBuildInitSegment_Structure(t *testing.T) {
	sps := []byte{0x67, 0x4D, 0x40, 0x28, 0x96, 0x35, 0x40, 0xa0, 0x12, 0x80}
	pps := []byte{0x68, 0xee, 0x3c, 0x80}
	out := BuildInitSegment(InitSegmentParams{
		TrackID:   1,
		Timescale: 90000,
		Width:     1280,
		Height:    720,
		SPS:       sps,
		PPS:       pps,
	})

	//Top level: ftyp then moov.
	typ, sz, _ := readBox(t, out, 0)
	if typ != "ftyp" {
		t.Errorf("first box = %q, want ftyp", typ)
	}
	typ2, _, moovBody := readBox(t, out, int(sz))
	if typ2 != "moov" {
		t.Errorf("second box = %q, want moov", typ2)
	}

	//moov must contain mvhd, trak, mvex.
	for _, want := range []string{"mvhd", "trak", "mvex"} {
		if findBox(t, moovBody, want) == nil {
			t.Errorf("moov missing %s", want)
		}
	}

	//trak.mdia.minf.stbl.stsd.avc1.avcC must embed the SPS we passed.
	trakBody := findBox(t, moovBody, "trak")
	mdiaBody := findBox(t, trakBody, "mdia")
	minfBody := findBox(t, mdiaBody, "minf")
	stblBody := findBox(t, minfBody, "stbl")
	stsdBody := findBox(t, stblBody, "stsd")
	//stsd has 4-byte fullbox header + 4-byte entry_count, then SampleEntry.
	avc1 := stsdBody[8:]
	avc1Type := string(avc1[4:8])
	if avc1Type != "avc1" {
		t.Errorf("sample entry = %q, want avc1", avc1Type)
	}
	avc1Body := avc1[8:binary.BigEndian.Uint32(avc1[:4])]
	//avc1 is a VisualSampleEntry with a fixed 78-byte header before
	//sub-boxes start. Skip past it before findBox-ing avcC.
	const visualSampleEntryHeader = 78
	avcC := findBox(t, avc1Body[visualSampleEntryHeader:], "avcC")
	if avcC == nil {
		t.Fatal("avc1 missing avcC")
	}
	if !bytes.Contains(avcC, sps) {
		t.Errorf("avcC doesn't embed SPS: %x", avcC)
	}
	if !bytes.Contains(avcC, pps) {
		t.Errorf("avcC doesn't embed PPS: %x", avcC)
	}
}

func TestBuildMediaSegment_Structure(t *testing.T) {
	samples := []Sample{
		{Duration: 3000, Size: 100, IsKey: true, Data: bytes.Repeat([]byte{0xAA}, 100)},
		{Duration: 3000, Size: 50, IsKey: false, CompositionTimeOffset: 3000, Data: bytes.Repeat([]byte{0xBB}, 50)},
	}
	out := BuildMediaSegment(MediaSegmentParams{
		TrackID:        1,
		SequenceNumber: 7,
		BaseDecodeTime: 90000,
		Samples:        samples,
	})

	moofType, moofSize, moofBody := readBox(t, out, 0)
	if moofType != "moof" {
		t.Errorf("first box = %q, want moof", moofType)
	}
	mdatType, _, mdatBody := readBox(t, out, int(moofSize))
	if mdatType != "mdat" {
		t.Errorf("second box = %q, want mdat", mdatType)
	}
	if len(mdatBody) != 150 {
		t.Errorf("mdat body length = %d, want 150", len(mdatBody))
	}
	if !bytes.Equal(mdatBody[:100], bytes.Repeat([]byte{0xAA}, 100)) {
		t.Error("mdat first sample bytes wrong")
	}

	traf := findBox(t, moofBody, "traf")
	if traf == nil {
		t.Fatal("moof missing traf")
	}
	tfdt := findBox(t, traf, "tfdt")
	if tfdt == nil {
		t.Fatal("traf missing tfdt")
	}
	//tfdt body: 4-byte fullbox header, then 8-byte baseMediaDecodeTime
	//(version 1).
	gotBaseTime := binary.BigEndian.Uint64(tfdt[4:12])
	if gotBaseTime != 90000 {
		t.Errorf("tfdt baseMediaDecodeTime = %d, want 90000", gotBaseTime)
	}

	trun := findBox(t, traf, "trun")
	if trun == nil {
		t.Fatal("traf missing trun")
	}
	//trun body: 4-byte fullbox header (version 1, flags 0x000F01),
	//4-byte sample_count, 4-byte data_offset, then per-sample records.
	sampleCount := binary.BigEndian.Uint32(trun[4:8])
	if sampleCount != 2 {
		t.Errorf("trun sample_count = %d, want 2", sampleCount)
	}
	dataOffset := binary.BigEndian.Uint32(trun[8:12])
	if int(dataOffset) != int(moofSize)+8 {
		t.Errorf("trun data_offset = %d, want %d (moof size + mdat header)",
			dataOffset, int(moofSize)+8)
	}
}

func TestSampleFlags_KeyVsNonKey(t *testing.T) {
	if k, nk := avcSampleFlags(true), avcSampleFlags(false); k == nk {
		t.Errorf("key/non-key flags must differ; got %#x for both", k)
	}
}
