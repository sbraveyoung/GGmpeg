package libdash

import (
	"fmt"
	"strings"
	"time"
)

// manifestInputs bundles the DASH state needed to render a manifest.
// Caller copies values under the DASH mutex so the builder runs lock-
// free.
type manifestInputs struct {
	streamID          string
	availabilityStart time.Time
	timescale         uint32
	targetDur         time.Duration
	width             uint16
	height            uint16
	codecStr          string //RFC 6381 codec string; empty → avc1 fallback
	segments          []segmentInfo
}

// buildMPD emits a dynamic (live) MPEG-DASH manifest using
// SegmentTemplate with $Number$ substitution. Compatible with Shaka
// Player and dash.js out of the box.
//
// Trade-offs in this minimal implementation:
//   - Single Period, single AdaptationSet, single Representation
//   - Video only (audio TODO)
//   - Codec string is a fixed fallback ("avc1.42E01E"); a real impl
//     would derive profile/level from the SPS like the libmp4 avcC
//     box already does
func buildMPD(in manifestInputs) []byte {
	if len(in.segments) == 0 {
		return nil
	}

	targetDurSec := int(in.targetDur / time.Second)
	if targetDurSec < 1 {
		targetDurSec = 1
	}
	avail := in.availabilityStart.UTC().Format(time.RFC3339)
	startNumber := in.segments[0].seq

	//Per-segment durations vary slightly; pick the maximum to derive
	//target. SegmentTimeline gives byte-exact durations, but
	//SegmentTemplate with a fixed duration is widely supported and
	//simpler — players treat it as a hint, not a hard constraint.
	var maxSegDur uint64
	for _, s := range in.segments {
		if s.duration > maxSegDur {
			maxSegDur = s.duration
		}
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&sb, `<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" `+
		`type="dynamic" `+
		`profiles="urn:mpeg:dash:profile:isoff-live:2011" `+
		`minBufferTime="PT%ds" `+
		`availabilityStartTime="%s" `+
		`publishTime="%s" `+
		`minimumUpdatePeriod="PT%ds" `+
		`timeShiftBufferDepth="PT%ds" `+
		`>`+"\n",
		targetDurSec,
		avail,
		time.Now().UTC().Format(time.RFC3339),
		targetDurSec,
		targetDurSec*len(in.segments),
	)

	sb.WriteString(`  <Period id="0" start="PT0S">` + "\n")
	sb.WriteString(`    <AdaptationSet contentType="video" segmentAlignment="true" ` +
		`mimeType="video/mp4" startWithSAP="1">` + "\n")

	codecStr := in.codecStr
	if codecStr == "" {
		codecStr = "avc1.42E01E" //baseline @ level 3.0; conservative fallback
	}
	bandwidth := 1000000 //placeholder bps

	fmt.Fprintf(&sb, `      <Representation id="v0" codecs="%s" `+
		`bandwidth="%d" width="%d" height="%d" frameRate="30">`+"\n",
		codecStr, bandwidth, in.width, in.height)

	fmt.Fprintf(&sb, `        <SegmentTemplate `+
		`timescale="%d" `+
		`duration="%d" `+
		`startNumber="%d" `+
		`initialization="%s-init.mp4" `+
		`media="%s-$Number$.m4s"/>`+"\n",
		in.timescale,
		maxSegDur,
		startNumber,
		in.streamID, in.streamID,
	)

	sb.WriteString(`      </Representation>` + "\n")
	sb.WriteString(`    </AdaptationSet>` + "\n")
	sb.WriteString(`  </Period>` + "\n")
	sb.WriteString(`</MPD>` + "\n")

	return []byte(sb.String())
}
