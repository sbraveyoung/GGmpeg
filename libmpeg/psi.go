package mpeg

import (
	"fmt"

	"github.com/SmartBrave/Athena/easyio"
)

//https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.15.01_60/en_300468v011501p.pdf
//https://en.wikipedia.org/wiki/MPEG_transport_stream
//iso13818-1.pdf: Table 2-23 Program specific information
type PSI interface{}

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
}

func NewPAT() (pat *PAT) {
	return &PAT{
		PMTs: make(map[uint16]*PMT),
	}
}

func (pat *PAT) Parse(reader easyio.EasyReader) (err error) {
	b, err := reader.ReadN(8)
	if err != nil {
		return fmt.Errorf("read PAT error:%w", err)
	}

	pat.TableID = b[0]                                      //0x00
	pat.SectionSyntaxIndicator = (b[1] >> 7) & 0x01         //0x01
	pat.SectionLength = uint16(((b[1] & 0x0f) << 4) + b[2]) //0x0d == 13
	pat.TransportStreamID = uint16(b[3]<<8 + b[4])          //0x01
	pat.VersionNumber = (b[5] >> 1) & 0x1f                  //0x01
	pat.CurrentNextIndicator = b[5] & 0x01                  //0x01
	pat.SectionNumber = b[6]                                //0x00
	pat.LastSectionNumber = b[7]                            //0x00

	if pat.TableID != 0x00 {
		return fmt.Errorf("invalid TableID of PAT, TableID:%d", pat.TableID)
	}
	if pat.SectionSyntaxIndicator != 0x01 {
		return fmt.Errorf("invalid SectionSyntaxIndicator of PAT, SectionSyntaxIndicator:%d", pat.SectionSyntaxIndicator)
	}
	if (pat.SectionLength>>10)&0x03 != 0x00 || pat.SectionLength > 0x3fd {
		return fmt.Errorf("invalid SectionLength of PAT, SectionLength:%d", pat.SectionLength)
	}

	b, err = reader.ReadN(uint32(pat.SectionLength - 5 - 4)) //4:CRC_32
	if err != nil {
		return fmt.Errorf("read ProgramNUmber of PAT error:%w", err)
	}
	for i := 0; i+4 <= len(b); i += 4 { //167
		programNumber := uint16(b[i]<<8 + b[i+1])
		if programNumber == 0x0000 { //NIT, ignore
			continue
		} else { //PMT
			pmtPID := uint16(b[i+2]<<8 + b[i+3])
			if _, ok := pat.PMTs[pmtPID]; !ok {
				pat.PMTs[pmtPID] = NewPMT(programNumber)
			}
		}
	}

	b, err = reader.ReadN(4) //CRC_32
	if err != nil {
		return fmt.Errorf("read CRC_32 of PAT error:%w", err)
	}
	return nil
}

func (pat *PAT) Marshal(writer easyio.EasyWriter) (err error) {
	b := []byte{
		pat.TableID,
		((pat.SectionSyntaxIndicator & 0x01) << 7) | uint8((pat.SectionLength>>8)&0x0f),
		uint8(pat.SectionLength & 0xff),
		uint8((pat.TransportStreamID >> 8) & 0xff),
		uint8(pat.TransportStreamID & 0xff),
		((pat.VersionNumber & 0x1f) << 1) | (pat.CurrentNextIndicator & 0x01),
		pat.SectionNumber,
		pat.LastSectionNumber,
	}
	for pmtPID, pmt := range pat.PMTs {
		b = append(b, uint8(pmt.ProgramNumber>>8)&0xff, uint8(pmt.ProgramNumber&0xff))
		b = append(b, uint8(pmtPID>>8)&0xff, uint8(pmtPID&0xff))
	}
	//TODO: CRC_32
	b = append(b, 0xff, 0xff, 0xff, 0xff)

	return writer.WriteFull(b)
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
}

func NewPMT(programNumber uint16) (pmt *PMT) {
	return &PMT{
		ProgramNumber: programNumber,
		Streams:       make(map[uint16]*PES),
	}

}

func (pmt *PMT) Parse(reader easyio.EasyReader) (err error) {
	b, err := reader.ReadN(12)
	if err != nil {
		return fmt.Errorf("read PMT error:%w", err)
	}

	pmt.TableID = b[0]
	pmt.SectionSyntaxIndicator = (b[1] >> 7) & 0x01
	pmt.SectionLength = uint16(((b[1] & 0x0f) << 4) + b[2])
	pmt.ProgramNumber = uint16(b[3]<<8 + b[4])
	pmt.VersionNumber = (b[5] >> 1) & 0x1f
	pmt.CurrentNextIndicator = b[5] & 0x01
	pmt.SectionNumber = b[6]
	pmt.LastSectionNumber = b[7]
	pmt.PCR_PID = uint16((b[8]&0x1f)<<4 + b[9])
	pmt.ProgramInfoLength = uint16((b[10]&0x0f)<<4 + b[11])

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

	b, err = reader.ReadN(uint32(pmt.ProgramInfoLength))
	if err != nil {
		return fmt.Errorf("read descriptor of PMT error:%w", err)
	}

	b, err = reader.ReadN(uint32(pmt.SectionLength - 9 - pmt.ProgramInfoLength - 4)) //4:CRC_32
	if err != nil {
		return fmt.Errorf("read streams of PMT error:%w", err)
	}
	for i := 0; i+5 <= len(b); i += 5 {
		streamPID := uint16((b[i+1]&0x1f)<<8 + b[i+2])
		if stream, ok := pmt.Streams[streamPID]; !ok {
			pes := NewPES(b[i], streamPID, uint16((b[i+3]&0x0f)<<8+b[i+4]))

			if (pes.ESInfoLength>>10)&0x03 != 0x00 {
				return fmt.Errorf("invalid ESInfoLength of PMT, ESInfoLength:%d", pes.ESInfoLength)
			}

			i += int(pes.ESInfoLength)
			pmt.Streams[streamPID] = pes
		}
	}

	b, err = reader.ReadN(4) //CRC_32
	if err != nil {
		return fmt.Errorf("read CRC_32 of PMT error:%w", err)
	}
	//TODO: check sum of crc32
	return nil
}

type CAT struct{} //Conditional Access Table, PID:0x01
type NIT struct{} //Network Information Table
