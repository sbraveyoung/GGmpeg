package libsrt

import (
	"bytes"
	"testing"
)

func TestSeqDiff(t *testing.T) {
	cases := []struct {
		a, b uint32
		want int32
	}{
		{10, 5, 5},
		{5, 10, -5},
		{0, 0, 0},
		{0x7FFFFFFF, 0, -1}, //wraps around
		{1, 0x7FFFFFFF, 2},  //wraps the other way
	}
	for _, c := range cases {
		if got := SeqDiff(c.a, c.b); got != c.want {
			t.Errorf("SeqDiff(%d,%d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestACKBody_Roundtrip(t *testing.T) {
	a := ACKBody{
		LastAckedSeq:    100,
		RTT:             1500,
		AvailableBuffer: 8192,
	}
	body := a.Marshal()
	if len(body) != 28 {
		t.Fatalf("body length = %d, want 28", len(body))
	}
	//Manual decode to verify field positions.
	if body[3] != 100 {
		t.Errorf("byte 3 (lastAckedSeq LSB) = %d, want 100", body[3])
	}
}

func TestNAK_SingleAndRange(t *testing.T) {
	losses := []LossRange{
		{From: 5, To: 5},     //single
		{From: 10, To: 14},   //range
		{From: 100, To: 100}, //single
	}
	body := MarshalNAK(losses)
	got := ParseNAK(body)
	if len(got) != 3 {
		t.Fatalf("ParseNAK got %d ranges, want 3", len(got))
	}
	for i, want := range losses {
		if got[i] != want {
			t.Errorf("range %d = %v, want %v", i, got[i], want)
		}
	}
}

func TestNAK_RangeBitFlag(t *testing.T) {
	body := MarshalNAK([]LossRange{{From: 7, To: 9}})
	//First word should have top bit set (range start).
	if body[0]&0x80 == 0 {
		t.Errorf("range start byte 0 = %#x, expected top bit set", body[0])
	}
	//Second word is plain (range end).
	if body[4]&0x80 != 0 {
		t.Errorf("range end byte 0 = %#x, expected top bit clear", body[4])
	}
	//Round-trip with another NAK to confirm we never collapse a
	//range into single + single.
	got := ParseNAK(body)
	if len(got) != 1 || got[0].From != 7 || got[0].To != 9 {
		t.Errorf("round-trip failed: %v", got)
	}
}

// TestNAK_TruncatedRange checks that a malformed body (range start
// flag with no following end word) is handled without panicking.
func TestNAK_TruncatedRange(t *testing.T) {
	body := []byte{0x80, 0x00, 0x00, 0x05} //range-start word, but no end
	got := ParseNAK(body)
	if len(got) != 0 {
		t.Errorf("expected truncated NAK to yield 0 ranges, got %v", got)
	}
}

// TestNAK_Empty asserts MarshalNAK on an empty slice returns an empty
// body — the SRT spec allows zero-loss NAKs (used for keepalive in
// some implementations).
func TestNAK_Empty(t *testing.T) {
	if got := MarshalNAK(nil); !bytes.Equal(got, []byte{}) {
		t.Errorf("got %v, want empty slice", got)
	}
}
