package librtmp

import "testing"

func TestParseClientPorts(t *testing.T) {
	cases := []struct {
		in       string
		wantRTP  int
		wantRTCP int
		wantOK   bool
	}{
		{"RTP/AVP;unicast;client_port=5000-5001", 5000, 5001, true},
		{"RTP/AVP;client_port=5000-5001;unicast", 5000, 5001, true},
		{"RTP/AVP;unicast;client_port=4000", 4000, 4001, true},
		{"RTP/AVP;unicast", 0, 0, false},
		{"RTP/AVP;unicast;client_port=abc-def", 0, 0, false},
	}
	for _, c := range cases {
		gotRTP, gotRTCP, ok := parseClientPorts(c.in)
		if ok != c.wantOK || gotRTP != c.wantRTP || gotRTCP != c.wantRTCP {
			t.Errorf("parseClientPorts(%q) = (%d,%d,%v) want (%d,%d,%v)",
				c.in, gotRTP, gotRTCP, ok, c.wantRTP, c.wantRTCP, c.wantOK)
		}
	}
}

func TestOpenUDPPair_BothEvenAndOdd(t *testing.T) {
	socks, rtp, rtcp, err := openUDPPair()
	if err != nil {
		t.Fatalf("openUDPPair: %v", err)
	}
	defer socks[0].Close()
	defer socks[1].Close()
	if rtp%2 != 0 {
		t.Errorf("RTP port %d should be even", rtp)
	}
	if rtcp != rtp+1 {
		t.Errorf("RTCP port = %d, want %d", rtcp, rtp+1)
	}
	if socks[0].LocalAddr().(interface{ Network() string }) == nil {
		t.Error("socks[0] not bound")
	}
}
