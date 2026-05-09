package librtsp

import (
	"bytes"
	"testing"
)

func TestParseSDP_AnnounceFromFFmpeg(t *testing.T) {
	//Adapted from a real `ffmpeg -f rtsp` ANNOUNCE body. Trimmed to
	//the lines our parser cares about; format is otherwise verbatim.
	sdp := []byte(
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=No Name\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=tool:libavformat\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=42C01F;sprop-parameter-sets=Z0LAH40NQFAe2AtwEBAUAAADAAQAAAMAyDxgxlg=,aM4xsg==\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP 97\r\n" +
			"a=rtpmap:97 mpeg4-generic/44100/2\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210\r\n" +
			"a=control:streamid=1\r\n",
	)
	parsed, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP: %v", err)
	}
	if !parsed.HasVideo || !parsed.HasAudio {
		t.Fatalf("HasVideo=%v HasAudio=%v", parsed.HasVideo, parsed.HasAudio)
	}
	if parsed.VideoControl != "streamid=0" {
		t.Errorf("video control = %q", parsed.VideoControl)
	}
	if parsed.AudioControl != "streamid=1" {
		t.Errorf("audio control = %q", parsed.AudioControl)
	}
	if parsed.AudioRate != 44100 || parsed.AudioChans != 2 {
		t.Errorf("audio rtpmap = %d/%d", parsed.AudioRate, parsed.AudioChans)
	}
	if len(parsed.SPS) == 0 {
		t.Errorf("SPS empty")
	}
	if len(parsed.PPS) == 0 {
		t.Errorf("PPS empty")
	}
	wantConfig := []byte{0x12, 0x10}
	if !bytes.Equal(parsed.AudioConfig, wantConfig) {
		t.Errorf("AudioConfig = %x, want %x", parsed.AudioConfig, wantConfig)
	}
}

func TestParseSDP_VideoOnly(t *testing.T) {
	//SPS=[0x67 0x42 0xC0 0x1F] → Z0LAHw==; PPS=[0x68 0xCE 0x06 0xE2] → aM4G4g==
	sdp := []byte("v=0\r\nm=video 0 RTP/AVP 96\r\na=rtpmap:96 H264/90000\r\n" +
		"a=fmtp:96 sprop-parameter-sets=Z0LAHw==,aM4G4g==\r\n" +
		"a=control:trackID=0\r\n")
	parsed, err := ParseSDP(sdp)
	if err != nil {
		t.Fatalf("ParseSDP: %v", err)
	}
	if parsed.HasAudio {
		t.Errorf("expected no audio")
	}
	if len(parsed.SPS) == 0 || len(parsed.PPS) == 0 {
		t.Errorf("missing SPS/PPS")
	}
}

func TestParseSDP_NoMedia(t *testing.T) {
	if _, err := ParseSDP([]byte("v=0\r\ns=name\r\n")); err == nil {
		t.Errorf("expected error for SDP without media lines")
	}
}
