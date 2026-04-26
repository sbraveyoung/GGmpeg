package librtsp

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// SDPParams describes the media a publisher just made available — a
// single video track plus an optional audio track. The caller waits
// for the codec-specific sequence header(s) before generating SDP so
// fmtp lines are fully populated.
type SDPParams struct {
	StreamID string
	HasVideo bool
	HasAudio bool

	// Video. IsHEVC selects between H.264 (default) and H.265; VPS is
	// HEVC-only. SPS/PPS are NAL units (no start code, no length
	// prefix).
	IsHEVC bool
	VPS    []byte
	SPS    []byte
	PPS    []byte

	// Audio. IsOpus selects between AAC-hbr (default) and Opus.
	// AudioConfig is the AAC AudioSpecificConfig (ignored for Opus).
	IsOpus      bool
	AudioConfig []byte
	AudioRate   int //Hz
	AudioChans  int //1 or 2
}

// BuildSDP renders a minimal SDP body describing the live stream.
//
// The video m-line uses payload type 96 (a dynamic PT in the unassigned
// range 96-127) with profile-level-id and sprop-parameter-sets carrying
// the SPS+PPS, per RFC 6184 §8.2. The audio m-line uses payload type 97
// with mode=AAC-hbr per RFC 3640 §3.3.6.
func BuildSDP(p SDPParams) []byte {
	var sb strings.Builder
	sb.WriteString("v=0\r\n")
	sb.WriteString("o=- 0 0 IN IP4 127.0.0.1\r\n")
	fmt.Fprintf(&sb, "s=%s\r\n", p.StreamID)
	sb.WriteString("c=IN IP4 0.0.0.0\r\n")
	sb.WriteString("t=0 0\r\n")
	//RFC 4566 mentions a=control:* on session level so the client
	//knows where to send aggregate-control commands; per-track
	//a=control: lines on each m-line let SETUP target one track.
	sb.WriteString("a=control:*\r\n")

	if p.HasVideo {
		if p.IsHEVC && len(p.SPS) > 0 {
			writeHEVC(&sb, p.VPS, p.SPS, p.PPS)
		} else if len(p.SPS) >= 4 {
			writeVideo(&sb, p.SPS, p.PPS)
		}
	}
	if p.HasAudio {
		if p.IsOpus {
			writeOpus(&sb, p.AudioChans)
		} else if len(p.AudioConfig) >= 2 {
			writeAudio(&sb, p.AudioConfig, p.AudioRate, p.AudioChans)
		}
	}
	return []byte(sb.String())
}

// writeHEVC emits an H.265 m-line + fmtp per RFC 7798.
// sprop-vps / sprop-sps / sprop-pps carry base64-encoded NALs.
func writeHEVC(sb *strings.Builder, vps, sps, pps []byte) {
	sb.WriteString("m=video 0 RTP/AVP 96\r\n")
	sb.WriteString("a=rtpmap:96 H265/90000\r\n")
	fmt.Fprintf(sb, "a=fmtp:96 ")
	first := true
	if len(vps) > 0 {
		fmt.Fprintf(sb, "sprop-vps=%s", base64.StdEncoding.EncodeToString(vps))
		first = false
	}
	if len(sps) > 0 {
		if !first {
			sb.WriteString(";")
		}
		fmt.Fprintf(sb, "sprop-sps=%s", base64.StdEncoding.EncodeToString(sps))
		first = false
	}
	if len(pps) > 0 {
		if !first {
			sb.WriteString(";")
		}
		fmt.Fprintf(sb, "sprop-pps=%s", base64.StdEncoding.EncodeToString(pps))
	}
	sb.WriteString("\r\n")
	sb.WriteString("a=control:trackID=0\r\n")
}

// writeOpus emits an Opus m-line per RFC 7587. Clock is always 48000.
// Channels signalled via the rtpmap parameter; SDP usually carries
// a=fmtp:97 sprop-stereo=1 for 2-channel content.
func writeOpus(sb *strings.Builder, chans int) {
	if chans <= 0 {
		chans = 2
	}
	sb.WriteString("m=audio 0 RTP/AVP 97\r\n")
	fmt.Fprintf(sb, "a=rtpmap:97 opus/48000/%d\r\n", chans)
	if chans == 2 {
		sb.WriteString("a=fmtp:97 sprop-stereo=1\r\n")
	}
	sb.WriteString("a=control:trackID=1\r\n")
}

func writeVideo(sb *strings.Builder, sps, pps []byte) {
	sb.WriteString("m=video 0 RTP/AVP 96\r\n")
	sb.WriteString("a=rtpmap:96 H264/90000\r\n")
	//profile-level-id is the 3-byte profile/constraints/level prefix
	//of the SPS rendered as 6 hex digits (RFC 6184 §8.2.2).
	profileLevel := fmt.Sprintf("%02X%02X%02X", sps[1], sps[2], sps[3])
	spropSPS := base64.StdEncoding.EncodeToString(sps)
	spropPPS := base64.StdEncoding.EncodeToString(pps)
	fmt.Fprintf(sb, "a=fmtp:96 packetization-mode=1;profile-level-id=%s;sprop-parameter-sets=%s,%s\r\n",
		profileLevel, spropSPS, spropPPS)
	sb.WriteString("a=control:trackID=0\r\n")
}

func writeAudio(sb *strings.Builder, asc []byte, rate, chans int) {
	if rate <= 0 {
		rate = 44100
	}
	if chans <= 0 {
		chans = 2
	}
	fmt.Fprintf(sb, "m=audio 0 RTP/AVP 97\r\n")
	fmt.Fprintf(sb, "a=rtpmap:97 mpeg4-generic/%d/%d\r\n", rate, chans)
	//config= encodes the AudioSpecificConfig as hex; sizeLength=13 +
	//indexLength=3 + indexDeltaLength=3 are the AAC-hbr defaults.
	configHex := strings.ToUpper(fmt.Sprintf("%X", asc))
	fmt.Fprintf(sb, "a=fmtp:97 streamtype=5;profile-level-id=1;mode=AAC-hbr;"+
		"sizelength=13;indexlength=3;indexdeltalength=3;config=%s\r\n", configHex)
	sb.WriteString("a=control:trackID=1\r\n")
}
