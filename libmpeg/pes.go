package libmpeg

import (
	"fmt"

	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
)

type PES_TYPE uint8

const (
	I PES_TYPE = iota
	P
	B
)

//iso13818-1.pdf: 2.4.3.7
type PES struct {
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

func NewPES() (pes *PES) {
	return &PES{}
}

func (pes *PES) Parse(reader easyio.EasyReader) (err error) {
	b, err := reader.ReadN(6)
	if err != nil {
		return err
	}

	pes.PacketStartCodePrefix = (uint32(b[0]) << 16) | (uint32(b[1]) << 8) | (uint32(b[2]))
	pes.StreamID = b[3]
	pes.PESPacketLength = (uint16(b[4]) << 8) | (uint16(b[5]))

	//TODO
	//Table 2-18 Stream_id assignments
	if pes.StreamID != 0xbc && pes.StreamID != 0xbe && pes.StreamID != 0xbf && pes.StreamID != 0xf0 && pes.StreamID != 0xf1 && pes.StreamID != 0xff && pes.StreamID != 0xf2 && pes.StreamID != 0xf8 {
		b, err = reader.ReadN(3)
		if err != nil {
			return err
		}
		// pes.PESScramblingControl = (b[0] & 0x30) >> 4
		// pes.PESPriority = (b[0] & 0x08) >> 3
		// pes.DataAlignmentIndicator = (b[0] & 0x04) >> 2
		// pes.Copyright = (b[0] & 0x02) >> 1
		// pes.OriginalORCopy = b[0] & 0x01
		pes.PTS_DTSFlag = (b[1] & 0xc0) >> 6
		pes.ESCRFlag = (b[1] & 0x20) >> 5
		pes.ESRateFlag = (b[1] & 0x10) >> 4
		pes.DSMTrickModeFlag = (b[1] & 0x08) >> 3
		pes.AdditionalCopyInfoFlag = (b[1] & 0x04) >> 2
		pes.PESCRCFlag = (b[1] & 0x02) >> 1
		pes.PESExtensionFlag = b[1] & 0x01
		pes.PESHeaderDataLength = b[2]
		switch pes.PTS_DTSFlag {
		case 0x02:
			b, err = reader.ReadN(5)
			if err != nil {
				return err
			}
			pes.PTS = (uint64(b[4]&0xfe) >> 1) | (uint64(b[3]) << 7) | (uint64(b[2]&0xfe) << 14) | (uint64(b[1]) << 22) | (uint64(b[0]&0x0e) << 29)
		case 0x03:
			b, err = reader.ReadN(10)
			if err != nil {
				return err
			}
			pes.PTS = (uint64(b[4]&0xfe) >> 1) | (uint64(b[3]) << 7) | (uint64(b[2]&0xfe) << 14) | (uint64(b[1]) << 22) | (uint64(b[0]&0x0e) << 29)
			pes.DTS = (uint64(b[9]&0xfe) >> 1) | (uint64(b[8]) << 7) | (uint64(b[7]&0xfe) << 14) | (uint64(b[6]) << 22) | (uint64(b[5]&0x0e) << 29)
		default:
			//do nothing
		}
		if pes.ESCRFlag == 0x01 {
			b, err = reader.ReadN(6)
			if err != nil {
				return err
			}
			//pes.ESCRBase = (uint64(b[0]&0x38) << 27) | (uint64(b[0]&0x03) << 28) | (uint64(b[1]) << 20) | (uint64(b[2]&0xf8) << 12) | (uint64(b[2]&0x03) << 10) | (uint64(b[3]) << 2) | (uint64(b[4] & 0xf8))
			//pes.ESCRExtension = (uint16(b[4]&0x03) << 7) | (uint16(b[5] & 0xfe))
		}
		if pes.ESRateFlag == 0x01 {
			b, err = reader.ReadN(3)
			if err != nil {
				return err
			}
			//pes.ESRate = (uint32(b[0]&0x7f) << 14) | (uint32(b[1]) << 8) | (uint32(b[2]))
		}
		if pes.DSMTrickModeFlag == 0x01 {
			b, err = reader.ReadN(1)
			if err != nil {
				return err
			}
			trickModeControl := (b[0] & 0xe0) >> 5
			//TODO
			//Table 2-20 Trick mode control values
			switch trickModeControl {
			case 0x00: //fast forward,000
			case 0x01: //slow motion,001
			case 0x02: //freeze frame,010
			case 0x03: //fast reverse,011
			case 0x04: //slot reverse,100
			default: //reserved
			}
		}
		if pes.AdditionalCopyInfoFlag == 0x01 {
			b, err = reader.ReadN(1)
			if err != nil {
				return err
			}
			//TODO
		}
		if pes.PESCRCFlag == 0x01 {
			b, err = reader.ReadN(2)
			if err != nil {
				return err
			}
			//TODO
		}
		if pes.PESExtensionFlag == 0x01 {
			b, err = reader.ReadN(1)
			if err != nil {
				return err
			}
			pesPrivateDataFlag := (b[0] & 0x80) >> 7
			packHeaderFieldFlag := (b[0] & 0x40) >> 6
			programPacketSequenceCounterFlag := (b[0] & 0x20) >> 5
			pSTDBufferFlag := (b[0] & 0x10) >> 4
			pesExtensionFlag2 := b[0] & 0x01
			if pesPrivateDataFlag == 0x01 {
				b, err = reader.ReadN(16)
				if err != nil {
					return err
				}
				//TODO
			}
			if packHeaderFieldFlag == 0x01 {
				b, err = reader.ReadN(1)
				if err != nil {
					return err
				}
				//TODO
			}
			if programPacketSequenceCounterFlag == 0x01 {
				b, err = reader.ReadN(2)
				if err != nil {
					return err
				}
				//TODO
			}
			if pSTDBufferFlag == 0x01 {
				b, err = reader.ReadN(2)
				if err != nil {
					return err
				}
				//TODO
			}
			if pesExtensionFlag2 == 0x01 {
				b, err = reader.ReadN(1)
				if err != nil {
					return err
				}
				pesExtensionFieldLength := b[0]
				b, err = reader.ReadN(uint32(pesExtensionFieldLength))
				if err != nil {
					return err
				}
				//TODO
			}
			//TODO
		}
	} else if pes.StreamID == 0xbc || pes.StreamID == 0xbf || pes.StreamID == 0xf0 || pes.StreamID == 0xf1 || pes.StreamID == 0xff || pes.StreamID == 0xf2 || pes.StreamID == 0xf8 {
		b, err = reader.ReadN(uint32(pes.PESPacketLength))
		if err != nil {
			return err
		}
		//TODO: PES_packet_data_byte
	} else if pes.StreamID == 0xbe {
		b, err = reader.ReadN(uint32(pes.PESPacketLength))
		if err != nil {
			return err
		}
		//TODO: padding_byte
	}
	return nil
}

func (pes *PES) Marshal(writer easyio.EasyWriter, writable int) (n int, finish bool, err error) {
	fmt.Printf("writable:%d, pes.StreamID:%x, pts:%d, dts:%d, pes.Index:%d, len(pes.Data):%d\n", writable, pes.StreamID, pes.PTS, pes.DTS, pes.Index, len(pes.Data))
	var err1, err2 error
	var b []byte
	if pes.Index == 0 {
		b = []byte{
			uint8(pes.PacketStartCodePrefix >> 16),
			uint8(pes.PacketStartCodePrefix >> 8),
			uint8(pes.PacketStartCodePrefix),
			pes.StreamID,
			uint8(pes.PESPacketLength >> 8),
			uint8(pes.PESPacketLength),
		}
		if pes.StreamID != 0xbc && pes.StreamID != 0xbe && pes.StreamID != 0xbf && pes.StreamID != 0xf0 && pes.StreamID != 0xf1 && pes.StreamID != 0xff && pes.StreamID != 0xf2 && pes.StreamID != 0xf8 {
			b = append(b, 0x80) //ignore other useless fields
			if pes.PTS == pes.DTS {
				b = append(b, 0x80, 0x05)
				b = append(b, 0x21|(uint8(pes.PTS>>29)&0x0e), uint8(pes.PTS>>22), (uint8(pes.PTS>>14)&0xfe)|0x01, uint8(pes.PTS>>7), (uint8(pes.PTS<<1)&0xfe)|0x01)
			} else {
				b = append(b, 0xc0, 0x0a)
				b = append(b, 0x31|(uint8(pes.PTS>>29)&0x0e), uint8(pes.PTS>>22), (uint8(pes.PTS>>14)&0xfe)|0x01, uint8(pes.PTS>>7), (uint8(pes.PTS<<1)&0xfe)|0x01)
				b = append(b, 0x11|(uint8(pes.DTS>>29)&0x0e), uint8(pes.DTS>>22), (uint8(pes.DTS>>14)&0xfe)|0x01, uint8(pes.DTS>>7), (uint8(pes.DTS<<1)&0xfe)|0x01)
			}
		}

		if writable < len(b) {
			return 0, false, fmt.Errorf("invalid writable:%d with pes", writable)
		}
		err1 = writer.WriteFull(b)
		writable -= len(b)
	}

	if writable > len(pes.Data)-pes.Index {
		writable = len(pes.Data) - pes.Index
	}
	if pes.Index+writable > len(pes.Data) {
		writable = len(pes.Data) - pes.Index
	}

	n, err2 = writer.Write(pes.Data[pes.Index : pes.Index+writable])
	pes.Index += writable

	return len(b) + writable, pes.Index == len(pes.Data), easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}
