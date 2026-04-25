package librtmp

import (
	"crypto/hmac"
	"crypto/sha256"
	"testing"
)

// TestHandshakeDigestSpread pins the magic constant used in the
// Adobe digest-handshake offset math. The value is 728, derived from
// 764 (digest section) - 4 (offset marker) - 32 (digest itself).
func TestHandshakeDigestSpread(t *testing.T) {
	if handshakeDigestSpread != handshakeBlockSize-4-handshakeDigestSize {
		t.Errorf("digest spread = %d, want %d (= %d - 4 - %d)",
			handshakeDigestSpread, handshakeBlockSize-4-handshakeDigestSize,
			handshakeBlockSize, handshakeDigestSize)
	}
	if handshakeDigestSpread != 728 {
		t.Errorf("digest spread = %d, want 728", handshakeDigestSpread)
	}
}

// TestHandshakeS1DigestVerifies asserts that the digest we embed in S1
// is what an HMAC-SHA256 over the rest of S1 (keyed on the first 36
// bytes of FMSKey) would produce. Mirrors what the parsing code on the
// peer would do to verify our S1.
func TestHandshakeS1DigestVerifies(t *testing.T) {
	for _, mode := range []HandshakeMode{COMPLEX1, COMPLEX2} {
		p := &Peer{handshakeMode: mode}
		s1 := p.makeS1()
		digestBufOffset := 8
		if mode == COMPLEX1 {
			digestBufOffset = 8 + handshakeBlockSize
		}
		offset := (int(s1[digestBufOffset]) + int(s1[digestBufOffset+1]) +
			int(s1[digestBufOffset+2]) + int(s1[digestBufOffset+3])) % handshakeDigestSpread
		got := s1[digestBufOffset+4+offset : digestBufOffset+4+offset+handshakeDigestSize]

		joined := append([]byte{}, s1[:digestBufOffset+4+offset]...)
		joined = append(joined, s1[digestBufOffset+4+offset+handshakeDigestSize:]...)
		mac := hmac.New(sha256.New, FMSKey[:36])
		mac.Write(joined)
		want := mac.Sum(nil)
		if !hmac.Equal(got, want) {
			t.Errorf("mode %d: embedded S1 digest doesn't verify", mode)
		}
	}
}
