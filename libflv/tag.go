package libflv

// "fmt"

// "github.com/SmartBrave/Athena/easyio"

type Tag interface {
	Marshal() []byte
	GetTagInfo() *TagBase
	Data() []byte
}

type TagBase struct {
	TagType   uint8
	DataSize  uint32 //uint24
	TimeStamp uint32
	StreamID  uint32 //uint24, always 0
}

func (tb *TagBase) GetTagInfo() *TagBase {
	return tb
}

// func ParseTagBase(er easyio.EasyReader) (tb *TagBase, err error) {
// b, err := er.ReadN(11)
// if err != nil {
// return
// }

// tb = &TagBase{
// TagType:   b[0],
// DataSize:  uint32(0x00)<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]),
// TimeStamp: uint32(b[7])<<24 | uint32(b[4])<<16 | uint32(b[5])<<8 | uint32(b[6]),
// StreamID:  uint32(0x00)<<24 | uint32(b[8])<<16 | uint32(b[9])<<8 | uint32(b[10]),
// }
// fmt.Printf("tb:%+v\n", *tb)
// return tb, err
// }

// func (tb *TagBase) Marshal() (b []byte) {
// b = make([]byte, 0, 11+len(tb.Data))

// b = append(b, tb.TagType)
// b = append(b, uint8(tb.DataSize>>16)&0xff, uint8(tb.DataSize>>8)&0xff, uint8(tb.DataSize&0xff))
// b = append(b, uint8(tb.TimeStamp>>16)&0xff, uint8(tb.TimeStamp>>8)&0xff, uint8(tb.TimeStamp&0xff))
// b = append(b, uint8(tb.TimeStamp>>24)&0xff)
// b = append(b, uint8(tb.StreamID>>16)&0xff, uint8(tb.StreamID>>8)&0xff, uint8(tb.StreamID&0xff))
// b = append(b, tb.Data...)
// return b
// }
