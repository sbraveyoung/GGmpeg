package libmpeg

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/SmartBrave/Athena/easyio"
)

func TestPAT_Parse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *PAT
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first",
			args: args{
				data: []byte{ //ts header: 0x47, 0x40, 0x00, 0x10, 0x00,
					0x00, 0xb0, 0x0d, 0x00, 0x01, 0xc1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00, 0x2a, 0xb1, 0x04, 0xb2},
			},
			want: &PAT{
				TableID:                0x00,
				SectionSyntaxIndicator: 0x01,
				SectionLength:          0x0d,
				TransportStreamID:      0x01,
				VersionNumber:          0x00,
				CurrentNextIndicator:   0x01,
				SectionNumber:          0x00,
				LastSectionNumber:      0x00,
				PMTs: map[uint16]*PMT{
					0x1000: &PMT{
						ProgramNumber: 0x0001,
						Streams:       make(map[uint16]*PES),
					},
				},
				CRC32: 0x2ab104b2,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		reader := easyio.NewEasyReader(bytes.NewBuffer(tt.args.data))
		pat := NewPAT()

		t.Run(tt.name, func(t *testing.T) {
			if err := pat.Parse(reader); (err != nil) != tt.wantErr {
				t.Errorf("PAT.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(pat, tt.want) {
				t.Errorf("PAT.Parse()\n got pat = %+v\nwant pat = %+v", pat, tt.want)
			}
		})
	}
}

func TestPAT_Marshal(t *testing.T) {
	tests := []struct {
		name    string
		pat     *PAT
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first",
			pat: &PAT{
				TableID:                0x00,
				SectionSyntaxIndicator: 0x01,
				SectionLength:          0x0d,
				TransportStreamID:      0x01,
				VersionNumber:          0x00,
				CurrentNextIndicator:   0x01,
				SectionNumber:          0x00,
				LastSectionNumber:      0x00,
				PMTs: map[uint16]*PMT{
					0x1000: &PMT{
						ProgramNumber: 0x0001,
						Streams:       make(map[uint16]*PES),
					},
				},
				CRC32: 0x2ab104b2,
			},
			want:    []byte{0x00, 0xb0, 0x0d, 0x00, 0x01, 0xc1, 0x00, 0x00, 0x00, 0x01, 0xf0, 0x00, 0x2a, 0xb1, 0x04, 0xb2},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer([]byte{})
			writer := easyio.NewEasyWriter(buf)
			_, err := tt.pat.Marshal(writer)
			if (err != nil) != tt.wantErr {
				t.Errorf("PAT.Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			data, err := ioutil.ReadAll(buf)
			if err != nil {
				t.Errorf("ioutil.ReadAll() error = %v", err)
			}
			if !bytes.Equal(data, tt.want) {
				t.Errorf("PAT.Marshal() got = %x, want %x", data, tt.want)
			}
		})
	}
}

func TestPMT_Parse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *PMT
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first",
			args: args{
				data: []byte{ //ts header: 0x47,0x50,0x00,0x10,0x00
					0x02, 0xb0, 0x17, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00, 0x1b, 0xe1, 0x00, 0xf0, 0x00, 0x0f, 0xe1, 0x01, 0xf0, 0x00, 0x2f, 0x44, 0xb9, 0x9b,
				},
			},
			want: &PMT{
				TableID:                0x02,
				SectionSyntaxIndicator: 0x01,
				SectionLength:          0x17,
				ProgramNumber:          0x01,
				VersionNumber:          0x00,
				CurrentNextIndicator:   0x01,
				SectionNumber:          0x00,
				LastSectionNumber:      0x00,
				PCR_PID:                0x0100,
				ProgramInfoLength:      0x00,
				Streams: map[uint16]*PES{
					0x0100: &PES{
						StreamType: 0x1b,
					},
					0x0101: &PES{
						StreamType: 0x0f,
					},
				},
				CRC32: 0x2f44b99b,
			},
		},
	}
	for _, tt := range tests {
		reader := easyio.NewEasyReader(bytes.NewBuffer(tt.args.data))
		pmt := NewPMT(0)

		t.Run(tt.name, func(t *testing.T) {
			if err := pmt.Parse(reader); (err != nil) != tt.wantErr {
				t.Errorf("PMT.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(pmt, tt.want) {
				t.Errorf("PMT.Parse()\n got pmt = %+v\nwant pmt = %+v", pmt, tt.want)
			}
		})
	}
}

func TestPMT_Marshal(t *testing.T) {
	tests := []struct {
		name    string
		pmt     *PMT
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first",
			pmt: &PMT{
				TableID:                0x02,
				SectionSyntaxIndicator: 0x01,
				SectionLength:          0x17,
				ProgramNumber:          0x01,
				VersionNumber:          0x00,
				CurrentNextIndicator:   0x01,
				SectionNumber:          0x00,
				LastSectionNumber:      0x00,
				PCR_PID:                0x0100,
				ProgramInfoLength:      0x00,
				Streams: map[uint16]*PES{
					0x0100: &PES{
						StreamType: 0x1b,
					},
					0x0101: &PES{
						StreamType: 0x0f,
					},
				},
				CRC32: 0x2f44b99b,
			},
			want:    []byte{0x02, 0xb0, 0x17, 0x00, 0x01, 0xc1, 0x00, 0x00, 0xe1, 0x00, 0xf0, 0x00, 0x1b, 0xe1, 0x00, 0xf0, 0x00, 0x0f, 0xe1, 0x01, 0xf0, 0x00, 0x2f, 0x44, 0xb9, 0x9b},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer([]byte{})
			writer := easyio.NewEasyWriter(buf)
			_, err := tt.pmt.Marshal(writer)
			if (err != nil) != tt.wantErr {
				t.Errorf("PMT.Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}

			data, err := ioutil.ReadAll(buf)
			if err != nil {
				t.Errorf("ioutil.ReadAll() error = %v", err)
			}
			if !bytes.Equal(data, tt.want) {
				t.Errorf("PMT.Marshal() got = %x, want %x", data, tt.want)
			}
		})
	}
}
