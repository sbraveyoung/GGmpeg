package librtsp

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// SDPParams describes the media a publisher just made available — a
// single H.264 video track and an optional AAC audio track. Without
// SPS/PPS we can't generate a fully-formed SDP, so the caller waits
// for the AVC sequence header before issuing DESCRIBE responses.
type SDPParams struct {
	StreamID    string
	HasVideo    bool
	HasAudio    bool
	SPS         []byte //one SPS NAL (no start code, no 4-byte length prefix)
	PPS         []byte //one PPS NAL
	AudioConfig []byte //AudioSpecificConfig (the AAC sequence header body)
	AudioRate   int    //sampling frequency, e.g. 44100
	AudioChans  int    //channel count (1 or 2)
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

	if p.HasVideo && len(p.SPS) >= 4 {
		writeVideo(&sb, p.SPS, p.PPS)
	}
	if p.HasAudio && len(p.AudioConfig) >= 2 {
		writeAudio(&sb, p.AudioConfig, p.AudioRate, p.AudioChans)
	}
	return []byte(sb.String())
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
