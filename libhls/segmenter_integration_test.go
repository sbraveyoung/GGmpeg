package libhls

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SmartBrave/Athena/broadcast"
	"github.com/sbraveyoung/GGmpeg/libflv"
)

// TestSegmenter_IntegrationFLVtoTS feeds a synthetic publisher stream
// through the full HLS pipeline (broadcast → Start → openSegment →
// rotate → finaliseCurrent) and asserts that segments land on disk
// with non-trivial content and a valid playlist.
//
// Synthesises:
//   - one AVC sequence header (with realistic 4-byte SPS/PPS)
//   - one AAC sequence header (LC, 44.1 kHz, stereo)
//   - alternating keyframes / inter frames with monotonic 33ms steps,
//     enough to span ≥2 target durations (2 s default → ≥6 s of media)
func TestSegmenter_IntegrationFLVtoTS(t *testing.T) {
	tmp := t.TempDir()
	hls := NewHls().WithStreamID("integ").WithDir(tmp)
	hls.targetDur = 300 * time.Millisecond //rotate quickly so the test stays fast

	bd := broadcast.NewBroadcast(2) //matches the 2 meta tags publishMeta sends
	publishMeta(t, bd)

	done := make(chan error, 1)
	go func() {
		done <- hls.Start(broadcast.NewBroadcastReader(bd))
	}()

	//Drive 80 frames at ~30 fps with an IDR every 5 frames (~165 ms).
	//With targetDur=300ms that gives us multiple rotations and the
	//small per-iteration sleep prevents the writer from outpacing
	//the broadcast reader (whose buffer is just `cap=2`).
	for i := 0; i < 80; i++ {
		ts := uint32(i * 33)
		if i%5 == 0 {
			bd.Reset()
			bd.Write(makeAVCKeyframe(ts))
		} else {
			bd.Write(makeAVCInterFrame(ts))
		}
		time.Sleep(time.Millisecond)
	}
	//Tell Start to wind down by closing the broadcast.
	bd.DisAlive()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("hls.Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hls.Start did not return within 5s")
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	tsCount := 0
	var totalBytes int64
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			tsCount++
			info, _ := e.Info()
			totalBytes += info.Size()
		}
	}
	if tsCount < 2 {
		t.Errorf("expected ≥2 .ts segments, got %d (%v)", tsCount, entries)
	}
	if totalBytes < 1000 {
		t.Errorf("segments total only %d bytes — segmenter likely produced empty files", totalBytes)
	}

	playlist := string(hls.Playlist())
	for _, want := range []string{"#EXTM3U", "#EXT-X-VERSION:3", "#EXTINF:", "integ-"} {
		if !strings.Contains(playlist, want) {
			t.Errorf("playlist missing %q\n%s", want, playlist)
		}
	}

	//Each ts file should start with a sync byte (0x47).
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".ts") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tmp, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if len(data) == 0 || data[0] != 0x47 {
			t.Errorf("%s: first byte = %#x, want 0x47", e.Name(), data[0])
		}
		if len(data)%188 != 0 {
			t.Errorf("%s: size %d not a multiple of 188", e.Name(), len(data))
		}
	}
}

// TestSegmenter_RollingWindowReap drives more segments than fit in the
// rolling window and verifies that the oldest .ts files are reaped
// from disk (matches windowSize default of 6).
func TestSegmenter_RollingWindowReap(t *testing.T) {
	tmp := t.TempDir()
	hls := NewHls().WithStreamID("roll").WithDir(tmp)
	hls.targetDur = 200 * time.Millisecond //rotate quickly
	hls.windowSize = 3                     //tighter window

	bd := broadcast.NewBroadcast(2)
	publishMeta(t, bd)
	done := make(chan error, 1)
	go func() { done <- hls.Start(broadcast.NewBroadcastReader(bd)) }()

	//~80 frames @ 33ms over a 200 ms target should give us ~13
	//rotations — well past the windowSize of 3.
	for i := 0; i < 80; i++ {
		ts := uint32(i * 33)
		if i%4 == 0 {
			bd.Reset()
			bd.Write(makeAVCKeyframe(ts))
		} else {
			bd.Write(makeAVCInterFrame(ts))
		}
	}
	bd.DisAlive()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hls.Start hang")
	}

	entries, _ := os.ReadDir(tmp)
	tsCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ts") {
			tsCount++
		}
	}
	if tsCount > hls.windowSize+1 { //+1 because finaliseCurrent always appends
		t.Errorf("disk holds %d segments, expected ≤%d (windowSize+1)",
			tsCount, hls.windowSize+1)
	}
}

// publishMeta sends the AVC + AAC sequence headers as FLV meta tags
// into the broadcast so toPES has decoder configuration before it
// sees real samples.
func publishMeta(t *testing.T, bd *broadcast.Broadcast) {
	t.Helper()
	sps := []byte{0x67, 0x42, 0xC0, 0x1E, 0x91, 0x40}
	pps := []byte{0x68, 0xCE, 0x06, 0xE2}
	dcr := []byte{
		0x01, 0x42, 0xC0, 0x1E,
		0xFF, 0xE1,
		byte(len(sps) >> 8), byte(len(sps)),
	}
	dcr = append(dcr, sps...)
	dcr = append(dcr, 0x01)
	dcr = append(dcr, byte(len(pps)>>8), byte(len(pps)))
	dcr = append(dcr, pps...)

	vt := &libflv.VideoTag{
		TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: 0},
		FrameType:     libflv.KEY_FRAME,
		CodecID:       libflv.FLV_VIDEO_AVC,
		AVCPacketType: libflv.AVC_SEQUENCE_HEADER,
		VideoData:     dcr,
	}
	vt.DataSize = uint32(len(vt.Data()))
	bd.WriteMeta(vt)

	asc := []byte{0x12, 0x10}
	at := &libflv.AudioTag{
		TagBase:       libflv.TagBase{TagType: libflv.AUDIO_TAG, TimeStamp: 0},
		SoundFormat:   libflv.FLV_AUDIO_AAC,
		SoundRate:     3,
		SoundSize:     libflv.SND_16_BIT,
		SoundType:     libflv.SND_STEREO,
		AACPacketType: libflv.AAC_SEQUENCE_HEADER,
		SoundData:     asc,
	}
	at.DataSize = uint32(len(at.Data()))
	bd.WriteMeta(at)
}

// makeAVCKeyframe / makeAVCInterFrame return synthetic AVCC video tags.
// Data shape: 4-byte length + tiny NAL body. NAL type 5 = IDR for
// keyframes, 1 = non-IDR slice for inter frames. AnnexB-prefixed so
// libavc.Parser.IsNaluHeader returns true and the segmenter passes the
// bytes through without conversion.
func makeAVCKeyframe(ts uint32) *libflv.VideoTag {
	body := append([]byte{0x00, 0x00, 0x00, 0x01, 0x65}, byteFiller(40)...)
	vt := &libflv.VideoTag{
		TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: ts},
		FrameType:     libflv.KEY_FRAME,
		CodecID:       libflv.FLV_VIDEO_AVC,
		AVCPacketType: libflv.AVC_NALU,
		VideoData:     body,
	}
	vt.DataSize = uint32(len(vt.Data()))
	return vt
}

func makeAVCInterFrame(ts uint32) *libflv.VideoTag {
	body := append([]byte{0x00, 0x00, 0x00, 0x01, 0x61}, byteFiller(20)...)
	vt := &libflv.VideoTag{
		TagBase:       libflv.TagBase{TagType: libflv.VIDEO_TAG, TimeStamp: ts},
		FrameType:     libflv.INTER_FRAME,
		CodecID:       libflv.FLV_VIDEO_AVC,
		AVCPacketType: libflv.AVC_NALU,
		VideoData:     body,
	}
	vt.DataSize = uint32(len(vt.Data()))
	return vt
}

// byteFiller returns a deterministic n-byte filler (avoids all-zero
// runs that could be misinterpreted as start codes by AnnexB walkers).
func byteFiller(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(0x40 + (i % 0x40))
	}
	return out
}
