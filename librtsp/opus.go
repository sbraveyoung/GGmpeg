package librtsp

// Opus over RTP per RFC 7587. The model is dead simple: each RTP
// packet carries exactly one Opus packet, no AU-headers, no
// segmentation. Clock rate is fixed at 48 kHz regardless of the
// negotiated input rate, per §4.1.

// PackOpusFrame returns the RTP payload for one Opus packet — which is
// just the Opus packet bytes themselves. Provided as a named function
// (rather than an inline copy) so the call site reads symmetrically
// with PackAACFrame.
func PackOpusFrame(opusPacket []byte) []byte {
	if len(opusPacket) == 0 {
		return nil
	}
	out := make([]byte, len(opusPacket))
	copy(out, opusPacket)
	return out
}

// OpusClockRate is the RTP timestamp clock for Opus per RFC 7587 §4.1
// — always 48000, regardless of the encoder's actual sample rate.
const OpusClockRate = 48000
