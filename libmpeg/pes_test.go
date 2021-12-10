package libmpeg

import (
	"bytes"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/SmartBrave/Athena/easyio"
)

func TestPES_Parse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *PES
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "first",
			args: args{
				data: []byte{ //0x47, 0x41, 0x00, 0x30,
					0x07, 0x50, 0x00, 0x00, 0x7b, 0x0c, 0x7e, 0x00, 0x00, 0x00, 0x01, 0xe0, 0x00, 0x00, 0x80, 0x80,
					0x05, 0x21, 0x00, 0x07, 0xd8, 0x61, 0x00, 0x00, 0x00, 0x01, 0x09, 0xf0, 0x00, 0x00, 0x00, 0x01,
					0x67, 0x4d, 0x40, 0x28, 0xda, 0x02, 0xd0, 0x28, 0x68, 0x40, 0x00, 0x00, 0x03, 0x00, 0x40, 0x00,
					0x00, 0x07, 0xa3, 0xc6, 0x0c, 0xa8, 0x00, 0x00, 0x00, 0x01, 0x68, 0xef, 0x05, 0xf2, 0x00, 0x00,
					0x01, 0x65, 0x88, 0x82, 0x0b, 0x44, 0xf6, 0x68, 0x0b, 0xd0, 0x7f, 0x81, 0x44, 0x99, 0xf9, 0x1b,
					0x58, 0x69, 0x45, 0x52, 0xb9, 0xc7, 0x3a, 0x64, 0xa2, 0x25, 0x82, 0x8a, 0xc2, 0xb4, 0x61, 0x5d,
					0x19, 0x06, 0x13, 0x46, 0x1c, 0x3c, 0x31, 0x77, 0xf3, 0x62, 0xf0, 0xd9, 0xaa, 0x42, 0x81, 0x30,
					0xff, 0x5a, 0xef, 0x24, 0x84, 0x92, 0x23, 0x5f, 0x84, 0xa6, 0x70, 0x7a, 0x13, 0xa0, 0x13, 0x9e,
					0x0c, 0x22, 0xc1, 0x29, 0xf2, 0x9d, 0xc9, 0x60, 0x09, 0x23, 0x22, 0x6b, 0x96, 0x8c, 0x33, 0x53,
					0x1d, 0x4d, 0xe1, 0x87, 0x20, 0x4a, 0x0e, 0x24, 0x97, 0x9e, 0x23, 0x3d, 0x7a, 0xf3, 0xe8, 0x0b,
					0xd5, 0xae, 0x23, 0xca, 0xdb, 0x67, 0xca, 0xcd, 0xbb, 0xe0, 0x9e, 0x06, 0xa3, 0xc6, 0x07, 0x3a,
					0x23, 0xee, 0x7b, 0xb3, 0x95, 0xd4, 0x64, 0xa0,
				},
			},
			want:&PES{
	PacketStartCodePrefix uint32 //24bit
	StreamID              uint8  //8bit
	PESPacketLength       uint16 //16bit
	// Reversed1              uint8  //2bit,0x02
	PESScramblingControl   uint8 //2bit
	PESPriority            uint8 //1bit
	DataAlignmentIndicator uint8 //1bit
	Copyright              uint8 //1bit
	OriginalORCopy         uint8 //1bit
	PTS_DTSFlag            uint8 //2bit
	ESCRFlag               uint8 //1bit
	ESRateFlag             uint8 //1bit
	DSMTrickModeFlag       uint8 //1bit
	AdditionalCopyInfoFlag uint8 //1bit
	PESCRCFlag             uint8 //1bit
	PESExtensionFlag       uint8 //1bit
	PESHeaderDataLength    uint8 //8bit
	// Reversed2 uint8 //4bit, 0x02
	PTS           uint64 //33bit
	DTS           uint64 //33bit
	ESCRBase      uint64 //33bit
	ESCRExtension uint16 //9bit
	ESRate        uint32 //22bit
	Data          []byte
	Index         int

			}
		},
	}
	for _, tt := range tests {
		reader := easyio.NewEasyReader(bytes.NewBuffer(tt.args.data))
		pes := NewPES()

		t.Run(tt.name, func(t *testing.T) {
			if err := pes.Parse(reader); (err != nil) != tt.wantErr {
				t.Errorf("PES.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(pes, tt.want) {
				t.Errorf("PES.Parse()\n got pes = %+v\nwant pes = %+v", pes, tt.want)
			}
		})
	}
}

func TestPES_Marshal(t *testing.T) {
	type args struct {
		writable int
	}
	tests := []struct {
		name       string
		pes        *PES
		args       args
		want       []byte
		wantN      int
		wantFinish bool
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer([]byte{})
			writer := easyio.NewEasyWriter(buf)
			gotN, gotFinish, err := tt.pes.Marshal(writer, tt.args.writable)
			if (err != nil) != tt.wantErr {
				t.Errorf("PES.Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("PES.Marshal() gotN = %v, want %v", gotN, tt.wantN)
			}
			if gotFinish != tt.wantFinish {
				t.Errorf("PES.Marshal() gotFinish = %v, want %v", gotFinish, tt.wantFinish)
			}

			data, err := ioutil.ReadAll(buf)
			if err != nil {
				t.Errorf("ioutil.ReadAll() error = %v", err)
			}
			if !bytes.Equal(data, tt.want) {
				t.Errorf("PES.Marshal() got = %x, want %x", data, tt.want)
			}
		})
	}
}
