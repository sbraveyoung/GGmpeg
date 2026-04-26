// SRT data encryption hooks. Full key-management (KMREQ + KMRSP +
// PBKDF2-HMAC-SHA1 + AES Key Wrap RFC 3394) is intentionally out of
// scope — this file provides the AES-CTR primitive plus the IV
// derivation rule, which is the cryptographic core regardless of how
// the SEK is delivered.
//
// Wiring a real key-management path on top:
//
//	1. Receive a control packet with SubType == SRT_CMD_KMREQ.
//	2. PBKDF2-HMAC-SHA1(passphrase, salt[8:], 2048) → KEK (16 bytes).
//	3. AES-Unwrap(KEK, wrappedSEK) → SEK (16 bytes).
//	4. Stash SEK + salt; on each data packet with KK != 0 call
//	   DecryptDataPayload below.
//
// FFmpeg / OBS without a passphrase don't enter this path — KK = 0
// and packets are plaintext, which is what this library currently
// expects.
package libsrt

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
)

// CryptoState holds the SEKs (Stream Encryption Key) that decrypt the
// publisher's data packets. SRT supports two SEKs simultaneously
// (even/odd) so the publisher can rotate keys without dropping
// packets; KK in the data-packet message-info field selects which.
type CryptoState struct {
	EvenSEK [16]byte //KK = 0b01
	OddSEK  [16]byte //KK = 0b10
	Salt    [16]byte //14 bytes salt + 2 bytes nonce per spec
	Active  bool     //false → DecryptDataPayload is a no-op
}

// DecryptDataPayload decrypts one data packet's payload in place.
// keyIndicator is the KK field from the message-info word: 1 selects
// the even SEK, 2 the odd one. Returns the input unchanged when crypto
// isn't active.
func (cs *CryptoState) DecryptDataPayload(keyIndicator uint8, seqNumber uint32, payload []byte) ([]byte, error) {
	if !cs.Active || keyIndicator == 0 {
		return payload, nil
	}
	var key [16]byte
	switch keyIndicator {
	case 1:
		key = cs.EvenSEK
	case 2:
		key = cs.OddSEK
	default:
		return nil, errors.New("invalid SRT KK field")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	iv := buildIV(cs.Salt[:], seqNumber)
	stream := cipher.NewCTR(block, iv[:])
	out := make([]byte, len(payload))
	stream.XORKeyStream(out, payload)
	return out, nil
}

// buildIV derives the per-packet AES-CTR IV per draft-sharabayko-srt
// §6.1.5. Salt is 14 bytes; the trailing 2 bytes of the IV are zero
// (block counter gets incremented internally by the CTR mode).
//
//	IV[0..3]   = (sequence number) XOR salt[0..3]
//	IV[4..13]  = salt[4..13]
//	IV[14..15] = 0
func buildIV(salt []byte, seq uint32) [16]byte {
	var iv [16]byte
	if len(salt) >= 14 {
		copy(iv[0:14], salt[:14])
	}
	binary.BigEndian.PutUint32(iv[0:4],
		binary.BigEndian.Uint32(iv[0:4])^seq)
	return iv
}
