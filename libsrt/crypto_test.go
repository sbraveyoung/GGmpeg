package libsrt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestDecryptDataPayload_NoOpWhenInactive(t *testing.T) {
	cs := &CryptoState{}
	in := []byte{0xAA, 0xBB, 0xCC}
	got, err := cs.DecryptDataPayload(1, 0, in)
	if err != nil {
		t.Fatalf("DecryptDataPayload: %v", err)
	}
	if !bytes.Equal(got, in) {
		t.Errorf("inactive crypto must be passthrough; got %x want %x", got, in)
	}
}

func TestDecryptDataPayload_NoOpWhenKKZero(t *testing.T) {
	cs := &CryptoState{Active: true}
	in := []byte{0xDE, 0xAD}
	got, _ := cs.DecryptDataPayload(0, 99, in)
	if !bytes.Equal(got, in) {
		t.Errorf("KK=0 must skip decryption; got %x", got)
	}
}

func TestDecryptDataPayload_InvalidKK(t *testing.T) {
	cs := &CryptoState{Active: true}
	if _, err := cs.DecryptDataPayload(3, 0, []byte{}); err == nil {
		t.Error("expected error for KK=3")
	}
}

// TestDecryptDataPayload_RoundTrip verifies our decrypt undoes a
// matching AES-CTR encryption — the SRT-specific bit is the per-packet
// IV derivation, so we encrypt with the same recipe and check we get
// the plaintext back.
func TestDecryptDataPayload_RoundTrip(t *testing.T) {
	plain := []byte("the quick brown fox jumps over the lazy dog")
	cs := &CryptoState{Active: true}
	cs.EvenSEK = [16]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10,
	}
	cs.Salt = [16]byte{
		0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88,
		0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x00, 0x00,
	}
	const seq = uint32(0x12345)

	//Encrypt with the same IV recipe we'll later decrypt with.
	block, err := aes.NewCipher(cs.EvenSEK[:])
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	iv := buildIV(cs.Salt[:], seq)
	stream := cipher.NewCTR(block, iv[:])
	cipherText := make([]byte, len(plain))
	stream.XORKeyStream(cipherText, plain)

	got, err := cs.DecryptDataPayload(1, seq, cipherText)
	if err != nil {
		t.Fatalf("DecryptDataPayload: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip mismatch:\n got  %q\n want %q", got, plain)
	}
}

func TestBuildIV_SaltAndSeqMix(t *testing.T) {
	salt := bytes.Repeat([]byte{0xFF}, 14)
	const seq = uint32(0x00010203)
	iv := buildIV(salt, seq)
	//IV[0..3] should be salt[0..3] XOR seq.
	wantTop := [4]byte{0xFF ^ 0x00, 0xFF ^ 0x01, 0xFF ^ 0x02, 0xFF ^ 0x03}
	if !bytes.Equal(iv[:4], wantTop[:]) {
		t.Errorf("IV[0..3] = %x, want %x", iv[:4], wantTop)
	}
	//IV[14..15] always zero.
	if iv[14] != 0 || iv[15] != 0 {
		t.Errorf("IV trailer non-zero: %x", iv[14:])
	}
}
