package librtsp

import (
	"strings"
	"testing"
)

func TestBuildSDP_VideoAndAudio(t *testing.T) {
	sps := []byte{0x67, 0x42, 0xC0, 0x1E, 0xDB, 0x02, 0x80}
	pps := []byte{0x68, 0xCE, 0x06, 0xE2}
	asc := []byte{0x12, 0x10} //AAC-LC, 44.1 kHz, stereo placeholder
	body := string(BuildSDP(SDPParams{
		StreamID:    "live1",
		HasVideo:    true,
		HasAudio:    true,
		SPS:         sps,
		PPS:         pps,
		AudioConfig: asc,
		AudioRate:   44100,
		AudioChans:  2,
	}))

	wants := []string{
		"v=0\r\n",
		"s=live1\r\n",
		"m=video 0 RTP/AVP 96\r\n",
		"a=rtpmap:96 H264/90000\r\n",
		"profile-level-id=42C01E",        //from SPS bytes 1..3
		"sprop-parameter-sets=Z0LAHtsCgA==,aM4G4g==", //base64 of SPS,PPS
		"a=control:trackID=0\r\n",
		"m=audio 0 RTP/AVP 97\r\n",
		"a=rtpmap:97 mpeg4-generic/44100/2\r\n",
		"config=1210", //hex of asc bytes
		"a=control:trackID=1\r\n",
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("SDP missing %q\n--- full ---\n%s", w, body)
		}
	}
}

func TestBuildSDP_VideoOnly(t *testing.T) {
	body := string(BuildSDP(SDPParams{
		StreamID: "live1",
		HasVideo: true,
		SPS:      []byte{0x67, 0x42, 0xC0, 0x1E, 0xDB},
		PPS:      []byte{0x68, 0xCE, 0x06, 0xE2},
	}))
	if strings.Contains(body, "m=audio") {
		t.Errorf("video-only SDP shouldn't carry m=audio:\n%s", body)
	}
	if !strings.Contains(body, "m=video 0 RTP/AVP 96") {
		t.Errorf("video-only SDP missing video m-line:\n%s", body)
	}
}
