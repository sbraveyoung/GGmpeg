package libhls

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPlaylist_Empty(t *testing.T) {
	if got := buildPlaylist(nil); got != nil {
		t.Errorf("expected nil for empty segments, got %q", got)
	}
}

func TestBuildPlaylist_RollingWindow(t *testing.T) {
	segs := []segmentInfo{
		{filename: "x-3.ts", seq: 3, duration: 1.9},
		{filename: "x-4.ts", seq: 4, duration: 2.1},
		{filename: "x-5.ts", seq: 5, duration: 2.0},
	}
	got := string(buildPlaylist(segs))

	wants := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:3",
		"#EXT-X-TARGETDURATION:3", //ceil(2.1) = 3
		"#EXT-X-MEDIA-SEQUENCE:3", //oldest seq
		"#EXTINF:1.900,\nx-3.ts",
		"#EXTINF:2.100,\nx-4.ts",
		"#EXTINF:2.000,\nx-5.ts",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("playlist missing %q\n--- full ---\n%s", w, got)
		}
	}
	//A live playlist must NOT contain ENDLIST.
	if strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Errorf("live playlist must not contain ENDLIST:\n%s", got)
	}
}

func TestBuildPlaylist_TargetDurationFloor(t *testing.T) {
	//Sub-second segments should still produce TARGETDURATION >= 1.
	segs := []segmentInfo{{filename: "x-0.ts", seq: 0, duration: 0.4}}
	got := string(buildPlaylist(segs))
	if !strings.Contains(got, "#EXT-X-TARGETDURATION:1") {
		t.Errorf("target duration not floored to 1:\n%s", got)
	}
}

func TestBuildLLPlaylist_Tags(t *testing.T) {
	segs := []segmentInfo{
		{
			filename: "x-0.ts", seq: 0, duration: 2.0,
			parts: []partInfo{
				{duration: 0.333, byteOffset: 0, byteLength: 1000, independent: true},
				{duration: 0.333, byteOffset: 1000, byteLength: 900},
				{duration: 0.334, byteOffset: 1900, byteLength: 1100},
				{duration: 0.500, byteOffset: 3000, byteLength: 800},
				{duration: 0.500, byteOffset: 3800, byteLength: 850},
			},
		},
	}
	in := playlistInputs{
		segments:      segs,
		nextSeq:       1,
		currentName:   "x-1.ts",
		partTargetDur: 333 * time.Millisecond,
		currentParts: []partInfo{
			{duration: 0.333, byteOffset: 0, byteLength: 700, independent: true},
		},
	}
	got := string(buildLLPlaylist(in))

	wants := []string{
		"#EXT-X-VERSION:6",
		"#EXT-X-PART-INF:PART-TARGET=0.333",
		"#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=0.999",
		`#EXT-X-PART:DURATION=0.333,URI="x-0.ts",BYTERANGE="1000@0",INDEPENDENT=YES`,
		`#EXT-X-PART:DURATION=0.333,URI="x-0.ts",BYTERANGE="900@1000"`,
		`#EXTINF:2.000,` + "\nx-0.ts",
		`#EXT-X-PART:DURATION=0.333,URI="x-1.ts",BYTERANGE="700@0",INDEPENDENT=YES`,
		`#EXT-X-PRELOAD-HINT:TYPE=PART,URI="x-1.ts",BYTERANGE-START=700`,
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("LL playlist missing %q\n--- full ---\n%s", w, got)
		}
	}
	//ENDLIST not allowed in live LL-HLS playlist either.
	if strings.Contains(got, "#EXT-X-ENDLIST") {
		t.Error("LL playlist must not contain ENDLIST")
	}
}

func TestBuildLLPlaylist_NoSegments(t *testing.T) {
	in := playlistInputs{partTargetDur: 333 * time.Millisecond}
	if got := buildLLPlaylist(in); got != nil {
		t.Errorf("expected nil for empty inputs, got %q", got)
	}
}

func TestBuildLLPlaylist_OnlyInProgress(t *testing.T) {
	//No completed segments yet — but in-progress segment with one part.
	//Should still produce a playlist (preview).
	in := playlistInputs{
		nextSeq:       0,
		currentName:   "x-0.ts",
		partTargetDur: 333 * time.Millisecond,
		currentParts: []partInfo{
			{duration: 0.250, byteOffset: 0, byteLength: 500, independent: true},
		},
	}
	got := string(buildLLPlaylist(in))
	if !strings.Contains(got, `#EXT-X-PART:DURATION=0.250,URI="x-0.ts",BYTERANGE="500@0",INDEPENDENT=YES`) {
		t.Errorf("missing in-progress part:\n%s", got)
	}
	if !strings.Contains(got, `#EXT-X-PRELOAD-HINT:TYPE=PART,URI="x-0.ts",BYTERANGE-START=500`) {
		t.Errorf("missing preload hint:\n%s", got)
	}
}
