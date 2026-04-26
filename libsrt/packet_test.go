package libsrt

import "testing"

func TestParseHeader_Data(t *testing.T) {
	hdr := make([]byte, HeaderSize)
	MarshalDataHeader(hdr, 0x12345, 0x80000007, 0x123, 0xCAFE)
	got, body, err := ParseHeader(hdr)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if got.Kind != KindData {
		t.Errorf("kind = %v", got.Kind)
	}
	if got.SeqNumber != 0x12345 {
		t.Errorf("seq = %#x", got.SeqNumber)
	}
	if got.MessageInfo != 0x80000007 {
		t.Errorf("msgInfo = %#x", got.MessageInfo)
	}
	if got.Timestamp != 0x123 {
		t.Errorf("ts = %#x", got.Timestamp)
	}
	if got.DestSocketID != 0xCAFE {
		t.Errorf("dst = %#x", got.DestSocketID)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(body))
	}
}

func TestParseHeader_Control(t *testing.T) {
	hdr := make([]byte, HeaderSize+HandshakeBodySize)
	MarshalControlHeader(hdr, CtrlHandshake, 0, 0, 0, 0)
	got, body, err := ParseHeader(hdr)
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if got.Kind != KindControl {
		t.Errorf("kind = %v", got.Kind)
	}
	if got.ControlType != CtrlHandshake {
		t.Errorf("ctrlType = %v", got.ControlType)
	}
	if len(body) != HandshakeBodySize {
		t.Errorf("body length = %d, want %d", len(body), HandshakeBodySize)
	}
}

func TestParseHeader_TooShort(t *testing.T) {
	if _, _, err := ParseHeader([]byte{0x00, 0x01}); err == nil {
		t.Error("expected error on short input")
	}
}

func TestHandshake_Roundtrip(t *testing.T) {
	h := Handshake{
		Version:        srtVersion5,
		ExtensionField: 0x4A17,
		HandshakeType:  HSTypeAgreement,
		SrtSocketID:    0xDEADBEEF,
		SyncCookie:     0xABBAABBA,
	}
	body := h.Marshal()
	if len(body) != HandshakeBodySize {
		t.Fatalf("body length = %d", len(body))
	}
	got, err := ParseHandshake(body)
	if err != nil {
		t.Fatalf("ParseHandshake: %v", err)
	}
	if got.Version != h.Version || got.HandshakeType != h.HandshakeType ||
		got.SrtSocketID != h.SrtSocketID || got.SyncCookie != h.SyncCookie {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, h)
	}
}
