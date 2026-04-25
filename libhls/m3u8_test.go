package libhls

import (
	"strings"
	"testing"
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
