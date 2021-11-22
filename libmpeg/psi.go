package libmpeg

import (
	"encoding/binary"
	"fmt"

	"github.com/SmartBrave/Athena/easyio"
)

//https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
//https://en.wikipedia.org/wiki/MPEG_transport_stream
//iso13818-1.pdf: Table 2-23 Program specific information
type PSI interface {
	Parse(easyio.EasyReader) error
	Marshal(easyio.EasyWriter, int) (int, bool, error)
}

//iso13818-1.pdf: 2.4.4.3
type PAT struct { //Program Association Table, PID:0x0000
	TableID                uint8 //8bit, 0x00
	SectionSyntaxIndicator uint8 //1bit, 0x01
	// ZeroBit                uint8  //1bit, 0x0
	// Reversed1              uint8  //2bit
	SectionLength     uint16 //12bit
	TransportStreamID uint16 //16bit
	// Reversed2              uint8  //2bit
	VersionNumber        uint8 //5bit
	CurrentNextIndicator uint8 //1bit
	SectionNumber        uint8 //8bit
	LastSectionNumber    uint8 //8bit
	PMTs                 map[uint16]*PMT
	CRC32                uint32
}

func NewPAT() *PAT {
	return &PAT{
		PMTs: make(map[uint16]*PMT),
	}
}

func (pat *PAT) Parse(reader easyio.EasyReader) (err error) {
	b1 := make([]byte, 8)
	err = reader.ReadFull(b1)
	if err != nil {
		return fmt.Errorf("read PAT error:%w", err)
	}

	pat.TableID = b1[0]
	pat.SectionSyntaxIndicator = (b1[1] & 0x80) >> 7
	pat.SectionLength = uint16(b1[1]&0x0f)<<8 + uint16(b1[2])
	pat.TransportStreamID = uint16(b1[3])<<8 + uint16(b1[4])
	pat.VersionNumber = (b1[5] & 0x3e) >> 1
	pat.CurrentNextIndicator = b1[5] & 0x01
	pat.SectionNumber = b1[6]
	pat.LastSectionNumber = b1[7]

	if pat.TableID != 0x00 {
		return fmt.Errorf("invalid TableID of PAT, TableID:%d", pat.TableID)
	}
	if pat.SectionSyntaxIndicator != 0x01 {
		return fmt.Errorf("invalid SectionSyntaxIndicator of PAT, SectionSyntaxIndicator:%d", pat.SectionSyntaxIndicator)
	}
	if (pat.SectionLength>>10)&0x03 != 0x00 || pat.SectionLength > 0x3fd {
		return fmt.Errorf("invalid SectionLength of PAT, SectionLength:%d", pat.SectionLength)
	}

	b2 := make([]byte, pat.SectionLength-5-4) //4:CRC_32
	err = reader.ReadFull(b2)
	if err != nil {
		return fmt.Errorf("read ProgramNUmber of PAT error:%w", err)
	}
	for i := 0; i+4 <= len(b2); i += 4 {
		programNumber := uint16(b2[i])<<8 + uint16(b2[i+1])
		if programNumber == 0x0000 { //NIT, ignore
			continue
		} else { //PMT
			pmtPID := uint16(b2[i+2]&0x1f)<<8 + uint16(b2[i+3])
			if _, ok := pat.PMTs[pmtPID]; !ok {
				pat.PMTs[pmtPID] = NewPMT(programNumber)
			}
		}
	}

	pat.CRC32 = CRC32(append(b1, b2...))

	b3 := make([]byte, 4)
	err = reader.ReadFull(b3) //CRC_32
	if err != nil {
		return fmt.Errorf("read CRC_32 of PAT error:%w", err)
	}
	if got := binary.BigEndian.Uint32(b3); got != pat.CRC32 {
		return fmt.Errorf("crc error, want:%d, got:%d", pat.CRC32, got)
	}
	return nil
}

func (pat *PAT) Marshal(writer easyio.EasyWriter, writable int) (n int, finish bool, err error) {
	b := []byte{
		pat.TableID,
		((pat.SectionSyntaxIndicator << 7) & 0x80) | 0x30 | uint8((pat.SectionLength>>8)&0x0f),
		uint8(pat.SectionLength & 0xff),
		uint8((pat.TransportStreamID >> 8) & 0xff),
		uint8(pat.TransportStreamID & 0xff),
		0xc0 | ((pat.VersionNumber << 1) & 0x37) | (pat.CurrentNextIndicator & 0x01),
		pat.SectionNumber,
		pat.LastSectionNumber,
	}
	for pmtPID, pmt := range pat.PMTs {
		b = append(b, uint8(pmt.ProgramNumber>>8)&0xff, uint8(pmt.ProgramNumber&0xff))
		b = append(b, 0xe0|uint8((pmtPID>>8)&0xff), uint8(pmtPID&0xff))
	}

	// crc:=pat.CRC32
	crc := CRC32(b)
	b = append(b, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))

	if writable < len(b) {
		return 0, false, fmt.Errorf("invalid writable:%d with pat", writable)
	}
	return len(b), true, writer.WriteFull(b)
}

type PMT struct { //Program Map Table
	TableID                uint8 //8bit, 0x02
	SectionSyntaxIndicator uint8 //1bit
	// ZeroBit                uint8  //1bit
	// Reversed1              uint8  //2bit
	SectionLength uint16 //12bit
	ProgramNumber uint16 //16bit
	// Reversed2              uint8  //2bit
	VersionNumber        uint8 //5bit
	CurrentNextIndicator uint8 //1bit
	SectionNumber        uint8 //8bit
	LastSectionNumber    uint8 //8bit
	// Reserved3              uint8  //3bit
	PCR_PID uint16 //13bit
	// Reserved4              uint8  //4bit
	ProgramInfoLength uint16 //12bit
	Streams           map[uint16]*PES
	CRC32             uint32
}

func NewPMT(programNumber uint16) *PMT {
	return &PMT{
		ProgramNumber: programNumber,
		Streams:       make(map[uint16]*PES),
	}

}

func (pmt *PMT) Parse(reader easyio.EasyReader) (err error) {
	b1 := make([]byte, 12)
	err = reader.ReadFull(b1)
	if err != nil {
		return fmt.Errorf("read PMT error:%w", err)
	}

	pmt.TableID = b1[0]
	pmt.SectionSyntaxIndicator = (b1[1] & 0x80) >> 7
	pmt.SectionLength = uint16((b1[1]&0x0f)<<8) + uint16(b1[2])
	pmt.ProgramNumber = uint16(b1[3])<<8 + uint16(b1[4])
	pmt.VersionNumber = (b1[5] & 0x3e) >> 1
	pmt.CurrentNextIndicator = b1[5] & 0x01
	pmt.SectionNumber = b1[6]
	pmt.LastSectionNumber = b1[7]
	pmt.PCR_PID = uint16(b1[8]&0x1f)<<8 + uint16(b1[9])
	pmt.ProgramInfoLength = uint16((b1[10]&0x0f)<<8 + b1[11])

	if pmt.TableID != 0x02 {
		return fmt.Errorf("invalid TableID of PMT, TableID:%d", pmt.TableID)
	}
	if pmt.SectionSyntaxIndicator != 0x01 {
		return fmt.Errorf("invalid SectionSyntaxIndicator of PMT, SectionSyntaxIndicator:%d", pmt.SectionSyntaxIndicator)
	}
	if (pmt.SectionLength>>10)&0x03 != 0x00 || pmt.SectionLength > 0x3fd {
		return fmt.Errorf("invalid SectionLength of PMT, SectionLength:%d", pmt.SectionLength)
	}
	if pmt.SectionNumber != 0x00 {
		return fmt.Errorf("invalid SectionNumber of PMT, SectionNumber:%d", pmt.SectionNumber)
	}
	if pmt.LastSectionNumber != 0x00 {
		return fmt.Errorf("invalid LastSectionNumber of PMT, LastSectionNumber:%d", pmt.LastSectionNumber)
	}
	if (pmt.ProgramInfoLength>>10)&0x03 != 0x00 {
		return fmt.Errorf("invalid ProgramInfoLength of PMT, ProgramInfoLength:%d", pmt.ProgramInfoLength)
	}

	b2 := make([]byte, pmt.ProgramInfoLength)
	err = reader.ReadFull(b2)
	if err != nil {
		return fmt.Errorf("read descriptor of PMT error:%w", err)
	}

	b3 := make([]byte, pmt.SectionLength-9-pmt.ProgramInfoLength-4) //4:CRC_32
	err = reader.ReadFull(b3)
	if err != nil {
		return fmt.Errorf("read streams of PMT error:%w", err)
	}
	for i := 0; i+5 <= len(b3); i += 5 {
		streamPID := uint16(b3[i+1]&0x1f)<<8 + uint16(b3[i+2])
		if _, ok := pmt.Streams[streamPID]; !ok {
			esInfoLength := uint16(b3[i+3]&0x0f)<<8 + uint16(b3[i+4])
			if (esInfoLength>>10)&0x03 != 0x00 {
				return fmt.Errorf("invalid ESInfoLength of PMT, ESInfoLength:%d", esInfoLength)
			}
			i += int(esInfoLength)
			pmt.Streams[streamPID] = NewPES()
		}
	}

	pmt.CRC32 = CRC32(append(append(b1, b2...), b3...))

	b4 := make([]byte, 4)
	err = reader.ReadFull(b4) //CRC_32
	if err != nil {
		return fmt.Errorf("read CRC_32 of PMT error:%w", err)
	}

	if got := binary.BigEndian.Uint32(b4); got != pmt.CRC32 {
		return fmt.Errorf("crc error, want:%d, got:%d", pmt.CRC32, got)
	}
	return nil
}

func (pmt *PMT) Marshal(writer easyio.EasyWriter, writable int) (n int, finish bool, err error) {
	b := []byte{
		pmt.TableID,
		((pmt.SectionSyntaxIndicator << 7) & 0x80) | 0x30 | (uint8(pmt.SectionLength>>8) & 0x0f),
		uint8(pmt.SectionLength & 0xff),
		uint8(pmt.ProgramNumber>>8) & 0xff,
		uint8(pmt.ProgramNumber & 0xff),
		0xc0 | ((pmt.VersionNumber << 1) & 0x3e) | (pmt.CurrentNextIndicator & 0x01),
		pmt.SectionNumber,
		pmt.LastSectionNumber,
		0xe0 | uint8((pmt.PCR_PID>>8)&0x1f),
		uint8(pmt.PCR_PID & 0xff),
		0xf0 | uint8(pmt.ProgramInfoLength>>8)&0x0f,
		uint8(pmt.ProgramInfoLength & 0xff),
	}
	for streamPID, stream := range pmt.Streams {
		b = append(b, stream.StreamID)
		b = append(b, 0xe0|uint8(streamPID>>8)&0x1f, uint8(streamPID&0xff))
		b = append(b, 0xf0|uint8(0), uint8(0))
	}

	// crc:=pmt.CRC32
	crc := CRC32(b)
	b = append(b, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))

	if writable < len(b) {
		return 0, false, fmt.Errorf("invalid writable:%d with pmt", writable)
	}
	return len(b), true, writer.WriteFull(b)
}

type CAT struct{} //Conditional Access Table, PID:0x01
type NIT struct{} //Network Information Table
