package amf

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/SmartBrave/utils/easyio"
)

func TestAMF0_Decode(t *testing.T) {
	type args struct {
		r easyio.EasyReader
	}
	tests := []struct {
		name    string
		a       AMF0
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
			a := AMF0{}
			gotRes, err := a.Decode(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("AMF0.Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("AMF0.Decode() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeAMF0(t *testing.T) {
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
			gotMarker, gotI, err := decodeAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotMarker != tt.wantMarker {
				t.Errorf("decodeAMF0() gotMarker = %v, want %v", gotMarker, tt.wantMarker)
			}
			if !reflect.DeepEqual(gotI, tt.wantI) {
				t.Errorf("decodeAMF0() gotI = %v, want %v", gotI, tt.wantI)
			}
		})
	}
}

func Test_decodeNumberAMF0(t *testing.T) {
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
			gotNum, err := decodeNumberAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeNumberAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNum != tt.wantNum {
				t.Errorf("decodeNumberAMF0() = %v, want %v", gotNum, tt.wantNum)
			}
		})
	}
}

func Test_decodeBooleanAMF0(t *testing.T) {
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
			gotBoolean, err := decodeBooleanAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeBooleanAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBoolean != tt.wantBoolean {
				t.Errorf("decodeBooleanAMF0() = %v, want %v", gotBoolean, tt.wantBoolean)
			}
		})
	}
}

func Test_decodeStringAMF0(t *testing.T) {
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
			gotStr, err := decodeStringAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeStringAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStr != tt.wantStr {
				t.Errorf("decodeStringAMF0() = %v, want %v", gotStr, tt.wantStr)
			}
		})
	}
}

func Test_decodeLongStringAMF0(t *testing.T) {
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
			gotStr, err := decodeLongStringAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeLongStringAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStr != tt.wantStr {
				t.Errorf("decodeLongStringAMF0() = %v, want %v", gotStr, tt.wantStr)
			}
		})
	}
}

func Test_decodeObjectAMF0(t *testing.T) {
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
			gotRes, err := decodeObjectAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeObjectAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeObjectAMF0() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeTypedObjectAMF0(t *testing.T) {
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
			gotClassName, gotRes, err := decodeTypedObjectAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeTypedObjectAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotClassName != tt.wantClassName {
				t.Errorf("decodeTypedObjectAMF0() gotClassName = %v, want %v", gotClassName, tt.wantClassName)
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeTypedObjectAMF0() gotRes = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeReferenceAMF0(t *testing.T) {
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
			gotIndex, err := decodeReferenceAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeReferenceAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("decodeReferenceAMF0() = %v, want %v", gotIndex, tt.wantIndex)
			}
		})
	}
}

func Test_decodeEcmaArrayAMF0(t *testing.T) {
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
			gotRes, err := decodeEcmaArrayAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeEcmaArrayAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeEcmaArrayAMF0() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeStrictArrayAMF0(t *testing.T) {
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
			gotRes, err := decodeStrictArrayAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeStrictArrayAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRes, tt.wantRes) {
				t.Errorf("decodeStrictArrayAMF0() = %v, want %v", gotRes, tt.wantRes)
			}
		})
	}
}

func Test_decodeDateAMF0(t *testing.T) {
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
			gotDate, err := decodeDateAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeDateAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotDate, tt.wantDate) {
				t.Errorf("decodeDateAMF0() = %v, want %v", gotDate, tt.wantDate)
			}
		})
	}
}

func Test_decodeXMLDocumentAMF0(t *testing.T) {
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
			gotXml, err := decodeXMLDocumentAMF0(tt.args.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeXMLDocumentAMF0() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotXml, tt.wantXml) {
				t.Errorf("decodeXMLDocumentAMF0() = %v, want %v", gotXml, tt.wantXml)
			}
		})
	}
}
