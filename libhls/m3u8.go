package libhls

import (
	"fmt"
	"math"
	"strings"
)

type segmentInfo struct {
	filename string
	seq      int
	duration float64
	startDTS uint64
}

// buildPlaylist emits a live HLS (version 3) media playlist over the
// given rolling segment window. The newest segment is the last entry;
// readers pick up EXT-X-MEDIA-SEQUENCE from the oldest entry's seq.
func buildPlaylist(segments []segmentInfo) []byte {
	if len(segments) == 0 {
		return nil
	}
	var maxDur float64
	for _, s := range segments {
		if s.duration > maxDur {
			maxDur = s.duration
		}
	}
	target := int(math.Ceil(maxDur))
	if target < 1 {
		target = 1
	}
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:3\n")
	fmt.Fprintf(&sb, "#EXT-X-TARGETDURATION:%d\n", target)
	fmt.Fprintf(&sb, "#EXT-X-MEDIA-SEQUENCE:%d\n", segments[0].seq)
	sb.WriteString("#EXT-X-ALLOW-CACHE:NO\n")
	for _, s := range segments {
		fmt.Fprintf(&sb, "#EXTINF:%.3f,\n%s\n", s.duration, s.filename)
	}
	return []byte(sb.String())
}
