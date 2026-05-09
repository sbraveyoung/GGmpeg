package libhls

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type segmentInfo struct {
	filename string
	seq      int
	duration float64
	startDTS uint64
	parts    []partInfo //LL-HLS: empty when not in low-latency mode
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

// playlistInputs bundles everything buildLLPlaylist needs from the HLS
// instance. The caller is expected to have copied the values under
// hls.mu so the builder can run lock-free.
type playlistInputs struct {
	segments      []segmentInfo
	nextSeq       int
	currentParts  []partInfo
	currentName   string //empty when no in-progress segment
	partTargetDur time.Duration
}

// buildLLPlaylist renders the LL-HLS extensions on top of the regular
// playlist. References:
//   - draft-pantos-hls-rfc8216bis (in-progress IETF spec)
//   - Apple "HTTP Live Streaming 2nd Edition Transport" (LL-HLS guide)
//
// Tags emitted beyond the basic v3 set:
//   - EXT-X-VERSION:6 (required for partial-segment + byterange use)
//   - EXT-X-PART-INF:PART-TARGET=<seconds>
//   - EXT-X-SERVER-CONTROL with CAN-BLOCK-RELOAD=YES and PART-HOLD-BACK
//   - EXT-X-PART:DURATION=...,URI=...,BYTERANGE=...,INDEPENDENT=YES?
//   - EXT-X-PRELOAD-HINT:TYPE=PART,URI=...,BYTERANGE-START=...
//
// Players that don't speak LL-HLS will silently ignore the unknown
// tags and play the EXTINF entries as a regular live playlist.
func buildLLPlaylist(in playlistInputs) []byte {
	if len(in.segments) == 0 && in.currentName == "" {
		return nil
	}

	var maxDur float64
	for _, s := range in.segments {
		if s.duration > maxDur {
			maxDur = s.duration
		}
	}
	target := int(math.Ceil(maxDur))
	if target < 1 {
		target = 1
	}

	mediaSeq := 0
	if len(in.segments) > 0 {
		mediaSeq = in.segments[0].seq
	}

	partTargetSec := in.partTargetDur.Seconds()
	if partTargetSec <= 0 {
		partTargetSec = 0.333
	}
	//PART-HOLD-BACK must be at least 3 * PART-TARGET per spec.
	partHoldBack := partTargetSec * 3

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:6\n")
	fmt.Fprintf(&sb, "#EXT-X-TARGETDURATION:%d\n", target)
	fmt.Fprintf(&sb, "#EXT-X-MEDIA-SEQUENCE:%d\n", mediaSeq)
	fmt.Fprintf(&sb, "#EXT-X-PART-INF:PART-TARGET=%.3f\n", partTargetSec)
	fmt.Fprintf(&sb, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=%.3f\n", partHoldBack)

	//Emit each completed segment's parts then the EXTINF entry.
	for _, s := range in.segments {
		for _, p := range s.parts {
			writePartTag(&sb, p, s.filename)
		}
		fmt.Fprintf(&sb, "#EXTINF:%.3f,\n%s\n", s.duration, s.filename)
	}

	//Emit in-progress segment's parts (no EXTINF yet — segment isn't
	//closed). Followed by a PRELOAD-HINT pointing at the byte that
	//comes after the last completed part.
	var preloadOffset int64
	if in.currentName != "" {
		for _, p := range in.currentParts {
			writePartTag(&sb, p, in.currentName)
			if next := p.byteOffset + p.byteLength; next > preloadOffset {
				preloadOffset = next
			}
		}
		fmt.Fprintf(&sb, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=\"%s\",BYTERANGE-START=%d\n",
			in.currentName, preloadOffset)
	}

	return []byte(sb.String())
}

func writePartTag(sb *strings.Builder, p partInfo, uri string) {
	fmt.Fprintf(sb, "#EXT-X-PART:DURATION=%.3f,URI=\"%s\",BYTERANGE=\"%d@%d\"",
		p.duration, uri, p.byteLength, p.byteOffset)
	if p.independent {
		sb.WriteString(",INDEPENDENT=YES")
	}
	sb.WriteString("\n")
}
