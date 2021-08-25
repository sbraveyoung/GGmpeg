package libamf

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/SmartBrave/utils_sb/easyio"
)

func Test_amf0_Decode(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		a       AMF
		args    args
		wantRes []interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first",
			args: args{
				r: easyio.NewEasyReader(bytes.NewReader([]byte{
					0x02, 0x00, 0x0d, 0x40, 0x73, 0x65, 0x74, 0x44, 0x61, 0x74, 0x61, 0x46, 0x72, 0x61, 0x6d, 0x65,
					0x02, 0x00, 0x0a, 0x6f, 0x6e, 0x4d, 0x65, 0x74, 0x61, 0x44, 0x61, 0x74, 0x61, 0x08, 0x00, 0x00,
					0x00, 0x18, 0x00, 0x08, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x00, 0x40, 0x86, 0x80,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x00, 0x40, 0x94,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0d, 0x76, 0x69, 0x64, 0x65, 0x6f, 0x64, 0x61, 0x74,
					0x61, 0x72, 0x61, 0x74, 0x65, 0x00, 0x40, 0x68, 0x6a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09,
					0x66, 0x72, 0x61, 0x6d, 0x65, 0x72, 0x61, 0x74, 0x65, 0x00, 0x40, 0x2e, 0x00, 0x00, 0x00, 0x00,
				})),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := AMF0
			gotRes, err := a.Decode(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("amf0.Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("amf0.Decode() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name       string
		args       args
		wantMarker Marker
		wantI      interface{}
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMarker, gotI, err := decodeamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMarker != tt.wantMarker {
				t.Errorf("decodeamf0() gotMarker = %v, want %v", gotMarker, tt.wantMarker)
			}
			if !reflect.DeepEqual(gotI, tt.wantI) {
				t.Errorf("decodeamf0() gotI = %v, want %v", gotI, tt.wantI)
			}
		})
	}
}

func Test_decodeNumberamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantNum float64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, err := decodeNumberamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeNumberamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNum != tt.wantNum {
				t.Errorf("decodeNumberamf0() = %v, want %v", gotNum, tt.wantNum)
			}
		})
	}
}

func Test_decodeBooleanamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name        string
		args        args
		wantBoolean bool
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBoolean, err := decodeBooleanamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeBooleanamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBoolean != tt.wantBoolean {
				t.Errorf("decodeBooleanamf0() = %v, want %v", gotBoolean, tt.wantBoolean)
			}
		})
	}
}

func Test_decodeStringamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantStr string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, err := decodeStringamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeStringamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStr != tt.wantStr {
				t.Errorf("decodeStringamf0() = %v, want %v", gotStr, tt.wantStr)
			}
		})
	}
}

func Test_decodeLongStringamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantStr string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr, err := decodeLongStringamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeLongStringamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStr != tt.wantStr {
				t.Errorf("decodeLongStringamf0() = %v, want %v", gotStr, tt.wantStr)
			}
		})
	}
}

func Test_decodeObjectamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantRes map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := decodeObjectamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeObjectamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeObjectamf0() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeTypedObjectamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name          string
		args          args
		wantClassName string
		wantRes       map[string]interface{}
		wantErr       bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClassName, gotRes, err := decodeTypedObjectamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeTypedObjectamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotClassName != tt.wantClassName {
				t.Errorf("decodeTypedObjectamf0() gotClassName = %v, want %v", gotClassName, tt.wantClassName)
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeTypedObjectamf0() gotRes = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeReferenceamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name      string
		args      args
		wantIndex uint16
		wantErr   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, err := decodeReferenceamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeReferenceamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("decodeReferenceamf0() = %v, want %v", gotIndex, tt.wantIndex)
			}
		})
	}
}

func Test_decodeEcmaArrayamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantRes map[string]interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := decodeEcmaArrayamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeEcmaArrayamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeEcmaArrayamf0() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeStrictArrayamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantRes []interface{}
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRes, err := decodeStrictArrayamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeStrictArrayamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeStrictArrayamf0() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeDateamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name     string
		args     args
		wantDate time.Time
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDate, err := decodeDateamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeDateamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotDate, tt.wantDate) {
				t.Errorf("decodeDateamf0() = %v, want %v", gotDate, tt.wantDate)
			}
		})
	}
}

func Test_decodeXMLDocumentamf0(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		args    args
		wantXml []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotXml, err := decodeXMLDocumentamf0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeXMLDocumentamf0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotXml, tt.wantXml) {
				t.Errorf("decodeXMLDocumentamf0() = %v, want %v", gotXml, tt.wantXml)
			}
		})
	}
}

func Test_amf0_Encode(t *testing.T) {
	type args struct {
		w   easyio.EasyWriter
		obj interface{}
	}
	tests := []struct {
		name    string
		a       amf0
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := amf0{}
			if err := a.Encode(tt.args.w, tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("amf0.Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_encodeamf0(t *testing.T) {
	type args struct {
		w            easyio.EasyWriter
		obj          interface{}
		encodeMarker bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := encodeamf0(tt.args.w, tt.args.obj, tt.args.encodeMarker); (err != nil) != tt.wantErr {
				t.Errorf("encodeamf0() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_encodeNumberamf0(t *testing.T) {
	type args struct {
		w            easyio.EasyWriter
		num          float64
		encodeMarker bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := encodeNumberamf0(tt.args.w, tt.args.num, tt.args.encodeMarker); (err != nil) != tt.wantErr {
				t.Errorf("encodeNumberamf0() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_encodeBooleanamf0(t *testing.T) {
	type args struct {
		w            easyio.EasyWriter
		boolean      bool
		encodeMarker bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := encodeBooleanamf0(tt.args.w, tt.args.boolean, tt.args.encodeMarker); (err != nil) != tt.wantErr {
				t.Errorf("encodeBooleanamf0() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_encodeStringamf0(t *testing.T) {
	type args struct {
		w            easyio.EasyWriter
		str          string
		encodeMarker bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := encodeStringamf0(tt.args.w, tt.args.str, tt.args.encodeMarker); (err != nil) != tt.wantErr {
				t.Errorf("encodeStringamf0() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_encodeObjectamf0(t *testing.T) {
	type args struct {
		w            easyio.EasyWriter
		obj          reflect.Value
		encodeMarker bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := encodeObjectamf0(tt.args.w, tt.args.obj, tt.args.encodeMarker); (err != nil) != tt.wantErr {
				t.Errorf("encodeObjectamf0() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
