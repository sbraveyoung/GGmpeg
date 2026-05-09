package librtsp

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// ParsedSDP carries the bits of a publisher-supplied SDP (received in
// an ANNOUNCE body) that we need to reconstitute FLV sequence headers.
// Only H.264 + AAC tracks are populated; everything else is dropped.
type ParsedSDP struct {
	HasVideo       bool
	VideoControl   string //path appended to the base URL on SETUP
	VideoPayloadType uint8
	SPS            []byte //RBSP-stripped, no start code
	PPS            []byte

	HasAudio        bool
	AudioControl    string
	AudioPayloadType uint8
	AudioConfig     []byte //AAC AudioSpecificConfig
	AudioRate       int
	AudioChans      int
}

// ParseSDP extracts the fields we need from an SDP body. Returns nil
// (and a non-nil error) if neither track is recognisable.
func ParseSDP(body []byte) (*ParsedSDP, error) {
	out := &ParsedSDP{}
	var current string //"video" / "audio" / ""
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimRight(raw, "\r")
		if len(line) < 2 || line[1] != '=' {
			continue
		}
		key, value := line[0], line[2:]
		switch key {
		case 'm':
			//m=video 0 RTP/AVP 96  → current="video", payload=96
			fields := strings.Fields(value)
			if len(fields) < 4 {
				continue
			}
			pt, _ := strconv.Atoi(fields[3])
			switch fields[0] {
			case "video":
				current = "video"
				out.HasVideo = true
				out.VideoPayloadType = uint8(pt)
			case "audio":
				current = "audio"
				out.HasAudio = true
				out.AudioPayloadType = uint8(pt)
			default:
				current = ""
			}
		case 'a':
			parseAttribute(out, current, value)
		}
	}
	if !out.HasVideo && !out.HasAudio {
		return nil, fmt.Errorf("SDP has no recognised media")
	}
	return out, nil
}

func parseAttribute(out *ParsedSDP, current, value string) {
	switch {
	case strings.HasPrefix(value, "control:"):
		v := strings.TrimPrefix(value, "control:")
		switch current {
		case "video":
			out.VideoControl = v
		case "audio":
			out.AudioControl = v
		}
	case strings.HasPrefix(value, "fmtp:"):
		//fmtp:<pt> key=value;key=value
		rest := strings.TrimPrefix(value, "fmtp:")
		fields := strings.SplitN(rest, " ", 2)
		if len(fields) < 2 {
			return
		}
		params := parseFmtpParams(fields[1])
		switch current {
		case "video":
			if pseq, ok := params["sprop-parameter-sets"]; ok {
				sps, pps := splitSpropParameterSets(pseq)
				if len(sps) > 0 {
					out.SPS = sps
				}
				if len(pps) > 0 {
					out.PPS = pps
				}
			}
		case "audio":
			if cfg, ok := params["config"]; ok {
				if b, err := hex.DecodeString(cfg); err == nil {
					out.AudioConfig = b
				}
			}
		}
	case strings.HasPrefix(value, "rtpmap:"):
		//rtpmap:97 mpeg4-generic/44100/2
		rest := strings.TrimPrefix(value, "rtpmap:")
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			return
		}
		parts := strings.Split(fields[1], "/")
		if current == "audio" && len(parts) >= 2 {
			rate, _ := strconv.Atoi(parts[1])
			out.AudioRate = rate
			if len(parts) >= 3 {
				ch, _ := strconv.Atoi(parts[2])
				out.AudioChans = ch
			} else {
				out.AudioChans = 1
			}
		}
	}
}

// parseFmtpParams splits "key1=val1;key2=val2" into a map. Keys are
// lower-cased; values keep their original casing (config= is hex,
// case-insensitive in practice).
func parseFmtpParams(s string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(s, ";") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		eq := strings.Index(kv, "=")
		if eq < 0 {
			continue
		}
		out[strings.ToLower(kv[:eq])] = kv[eq+1:]
	}
	return out
}

// splitSpropParameterSets decodes one or more comma-separated base64
// NAL units. The first is conventionally the SPS, the second the PPS.
func splitSpropParameterSets(s string) (sps, pps []byte) {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		b, err := base64.StdEncoding.DecodeString(p)
		if err != nil {
			continue
		}
		switch i {
		case 0:
			sps = b
		case 1:
			pps = b
		}
	}
	return
}
