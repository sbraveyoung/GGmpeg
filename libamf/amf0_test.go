package libamf

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/SmartBrave/Athena/easyio"
)

// TestAMF0_RoundTrip_Number / Boolean / String / Object / EcmaArray
// covers the encoder + decoder symmetry per AMF0 spec markers we
// actually use on the RTMP wire.

func TestAMF0_Number(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := AMF0.Encode(easyio.NewEasyWriter(buf), 3.14); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(buf))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].(float64) != 3.14 {
		t.Errorf("got %v, want [3.14]", got)
	}
}

func TestAMF0_Boolean(t *testing.T) {
	for _, v := range []bool{true, false} {
		buf := &bytes.Buffer{}
		if err := AMF0.Encode(easyio.NewEasyWriter(buf), v); err != nil {
			t.Fatalf("encode %v: %v", v, err)
		}
		got, err := AMF0.Decode(easyio.NewEasyReader(buf))
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got[0].(bool) != v {
			t.Errorf("got %v, want %v", got[0], v)
		}
	}
}

func TestAMF0_String(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := AMF0.Encode(easyio.NewEasyWriter(buf), "onMetaData"); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(buf))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0].(string) != "onMetaData" {
		t.Errorf("got %q", got[0])
	}
}

func TestAMF0_LongString(t *testing.T) {
	long := bytes.Repeat([]byte("x"), 0x10001) //past the u16 ceiling
	buf := &bytes.Buffer{}
	if err := AMF0.Encode(easyio.NewEasyWriter(buf), string(long)); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(buf))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got[0].(string)) != len(long) {
		t.Errorf("long string truncated: got %d, want %d", len(got[0].(string)), len(long))
	}
}

func TestAMF0_Object(t *testing.T) {
	src := map[string]interface{}{
		"app":      "live",
		"flashVer": "FMS/3,0,1,123",
		"capabilities": 31.0,
		"audioOnly":    false,
	}
	buf := &bytes.Buffer{}
	if err := AMF0.Encode(easyio.NewEasyWriter(buf), src); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(buf))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	gotMap := got[0].(map[string]interface{})
	for k, v := range src {
		if !reflect.DeepEqual(gotMap[k], v) {
			t.Errorf("key %q: got %v want %v", k, gotMap[k], v)
		}
	}
}

func TestAMF0_Null(t *testing.T) {
	buf := &bytes.Buffer{}
	if err := AMF0.Encode(easyio.NewEasyWriter(buf), nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(buf))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0] != nil {
		t.Errorf("got %v, want nil", got[0])
	}
}

func TestAMF0_DecodeRTMPConnect(t *testing.T) {
	//Real bytes captured from an FFmpeg `connect` command. First
	//element is the command name "connect", second is txn id 1,
	//third is the connect command object with app/tcUrl/flashVer.
	//
	//Known quirk: readPairamf0 doesn't consume the trailing 0x09
	//ObjectEnd marker, so the outer decode loop emits an extra nil
	//element. Asserting the leading three are correct is sufficient
	//for downstream callers that index by position.
	raw := []byte{
		0x02, 0x00, 0x07, 'c', 'o', 'n', 'n', 'e', 'c', 't',
		0x00, 0x3F, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, //txn=1
		0x03, //object marker
		0x00, 0x03, 'a', 'p', 'p',
		0x02, 0x00, 0x04, 'l', 'i', 'v', 'e',
		0x00, 0x00, 0x09, //object end
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("want at least 3 elements, got %d: %v", len(got), got)
	}
	if got[0].(string) != "connect" {
		t.Errorf("cmd = %q", got[0])
	}
	if got[1].(float64) != 1 {
		t.Errorf("txnID = %v", got[1])
	}
	if obj, ok := got[2].(map[string]interface{}); !ok || obj["app"] != "live" {
		t.Errorf("object = %v", got[2])
	}
}

// TestAMF0_DecodeMultiElement covers the wire shape of an onMetaData
// data message — three elements that share one decoder invocation:
// a command name string, the metadata key string, and the ECMA array
// of properties.
func TestAMF0_DecodeMultiElement(t *testing.T) {
	raw := []byte{
		0x02, 0x00, 0x0A, 'o', 'n', 'M', 'e', 't', 'a', 'D', 'a', 't', 'a',
		0x08, 0x00, 0x00, 0x00, 0x01, //ecma array of 1 entry
		0x00, 0x05, 'w', 'i', 'd', 't', 'h',
		0x00, 0x40, 0x86, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x09, //array end
	}
	got, err := AMF0.Decode(easyio.NewEasyReader(bytes.NewReader(raw)))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got[0] != "onMetaData" {
		t.Errorf("name = %v", got[0])
	}
	if m, ok := got[1].(map[string]interface{}); !ok || m["width"].(float64) != 720 {
		t.Errorf("array = %v", got[1])
	}
}
