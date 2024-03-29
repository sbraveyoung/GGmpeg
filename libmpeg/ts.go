package libmpeg

import (
	"errors"
	"fmt"

	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
)

//https://ocw.unican.es/pluginfile.php/171/course/section/78/iso13818-1.pdf

const (
	PAT_PID   = 0x0000
	PMT_PID   = 0x1001
	AUDIO_PID = 0x0101
	VIDEO_PID = 0x0100
)

var (
	INVALID_DATA_ERROR = errors.New("invalid data")
)

type AdaptationField struct {
	AdaptationFieldLength                  uint8  //8bit
	DiscontinuityIndicator                 uint8  //1bit
	RandomAccessIndicator                  uint8  //1bit
	ElementaryStreamPriority               uint8  //1bit
	PCRFlag                                uint8  //1bit
	OPCRFlag                               uint8  //1bit
	SplicingPointFlag                      uint8  //1bit
	TransportPrivateDataFlag               uint8  //1bit
	AdaptationFieldExtensionFlag           uint8  //1bit
	ProgramClockReferenceBase              uint64 //33bit
	ProgramClockReferenceExtension         uint16 //9bit
	OriginalProgramClockReferenceBase      uint64 //33bit
	OriginalProgramClockReferenceExtension uint16 //9bit
	SpliceCountdown                        int8   //8bit
	TransportPrivateDataLength             uint8
	TransportPrivateData                   []byte
	AdaptationFieldExtensionLength         uint8  //8bit
	LTWFlag                                uint8  //1bit, Legal time window
	PiecewiseRateFlag                      uint8  //1bit
	SeamlessSpliceFlag                     uint8  //1bit
	LTWValidFlag                           uint8  //1bit
	LTWOffset                              uint16 //15bit
	PiecewiseRate                          uint32 //22bit
	SpliceType                             uint8  //4bit
	DTSNextAccessUnit                      uint64 //33bit
	// StuffingBytesLength                    int    //always 0xFF
}

func NewAdaptationField() (af *AdaptationField) {
	return &AdaptationField{}
}

func (af *AdaptationField) Parse(reader easyio.EasyReader) (err error) {
	b, err := reader.ReadN(1)
	if err != nil {
		return INVALID_DATA_ERROR
	}

	af.AdaptationFieldLength = b[0]

	if uint8(b[0]) == 0 {
		return nil
	}

	b, err = reader.ReadN(1)
	if err != nil {
		return INVALID_DATA_ERROR
	}

	af.DiscontinuityIndicator = (b[0] & 0x80) >> 7
	af.RandomAccessIndicator = (b[0] & 0x40) >> 6
	af.ElementaryStreamPriority = (b[0] & 0x20) >> 5
	af.PCRFlag = (b[0] & 0x10) >> 4
	af.OPCRFlag = (b[0] & 0x08) >> 3
	af.SplicingPointFlag = (b[0] & 0x04) >> 2
	af.TransportPrivateDataFlag = (b[0] & 0x02) >> 1
	af.AdaptationFieldExtensionFlag = b[0] & 0x01

	var err1, err2, err3, err4, err5, err6, err7, err8 error
	if af.PCRFlag == 1 {
		b, err1 = reader.ReadN(6)
		af.ProgramClockReferenceBase = (uint64(b[0]) << 25) | (uint64(b[1]) << 17) | (uint64(b[2]) << 9) | (uint64(b[3]) << 1) | (uint64(b[4]&0x80) >> 7)
		af.ProgramClockReferenceExtension = (uint16(b[4]&0x01) << 8) | (uint16(b[5]))
	}
	if af.OPCRFlag == 1 {
		b, err2 = reader.ReadN(6)
		af.OriginalProgramClockReferenceBase = (uint64(b[0]) << 25) | (uint64(b[1]) << 17) | (uint64(b[2]) << 9) | (uint64(b[3]) << 1) | (uint64(b[4]&0x80) >> 7)
		af.OriginalProgramClockReferenceExtension = (uint16(b[4]&0x01) << 8) | (uint16(b[5]))
	}
	if af.SplicingPointFlag == 1 {
		b, err3 = reader.ReadN(1)
		af.SpliceCountdown = int8(b[0])
	}
	if af.TransportPrivateDataFlag == 1 {
		b, err4 = reader.ReadN(1)
		af.TransportPrivateDataLength = b[0]
		af.TransportPrivateData, err5 = reader.ReadN(uint32(af.TransportPrivateDataLength))
	}
	if af.AdaptationFieldExtensionFlag == 1 {
		b, err5 = reader.ReadN(2)
		af.AdaptationFieldExtensionLength = b[0]
		af.LTWFlag = (b[1] & 0x80) >> 7
		af.PiecewiseRateFlag = (b[1] & 0x40) >> 6
		af.SeamlessSpliceFlag = (b[1] & 0x20) >> 5

		if af.LTWFlag == 1 {
			b, err6 = reader.ReadN(2)
			af.LTWValidFlag = (b[0] & 0x80) >> 7
			af.LTWOffset = uint16(b[0]&0x7f)<<8 + uint16(b[1])
		}
		if af.PiecewiseRateFlag == 1 {
			b, err7 = reader.ReadN(3)
			af.PiecewiseRate = uint32(b[0]&0xc0)<<16 + uint32(b[1])<<8 + uint32(b[2])
		}
		if af.SeamlessSpliceFlag == 1 {
			b, err8 = reader.ReadN(5)
			af.SpliceType = (b[0] & 0xf0) >> 4
			af.DTSNextAccessUnit = uint64(b[0]&0x0f)<<32 + uint64(b[1])<<24 + uint64(b[2])<<16 + uint64(b[3])<<8 + uint64(b[4])
		}
	}
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4, err5, err6, err7, err8)
}

func (af *AdaptationField) Marshal(writer easyio.EasyWriter) (n uint8, err error) {
	if af.AdaptationFieldLength < 1 {
		return 0, fmt.Errorf("invalid1 AdaptationFieldLength:%d", af.AdaptationFieldLength)
	}

	b := make([]byte, 0, af.AdaptationFieldLength+1)
	b = append(b, af.AdaptationFieldLength)
	b = append(b, ((af.DiscontinuityIndicator<<7)&0x80)|((af.RandomAccessIndicator<<6)&0x40)|((af.ElementaryStreamPriority<<5)&0x20)|((af.PCRFlag<<4)&0x10)|((af.OPCRFlag<<3)&0x08)|((af.SplicingPointFlag<<2)&0x04)|((af.TransportPrivateDataFlag<<1)&0x02)|(af.AdaptationFieldExtensionFlag&0x01))

	if af.PCRFlag == 1 {
		if af.AdaptationFieldLength < 7 {
			return 0, fmt.Errorf("invalid2 AdaptationFieldLength:%d", af.AdaptationFieldLength)
		}
		b = append(b, uint8(af.ProgramClockReferenceBase>>25), uint8(af.ProgramClockReferenceBase>>17), uint8(af.ProgramClockReferenceBase>>9), uint8(af.ProgramClockReferenceBase>>1), ((uint8(af.ProgramClockReferenceBase) & 0x01 << 7) | 0x7e), uint8(af.ProgramClockReferenceExtension) /*|(uint8(af.ProgramClockReferenceExtension)&0x01)*/)
	}

	var err1, err2 error

	err1 = writer.WriteFull(b)
	if len(b) < cap(b) {
		err2 = Fill0XFF(writer, cap(b)-len(b))
	}

	return af.AdaptationFieldLength + 1, easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}

type TS struct {
	SyncByte                   byte             //8bit
	TransportErrorIndicator    uint8            //1bit
	PayloadUnitStartIndicator  uint8            //1bit
	TransportPriority          uint8            //1bit
	PID                        uint16           //13bit
	TransportScramblingControl byte             //2bit
	AdaptationFieldExist       byte             //2bit
	ContinuityCounter          byte             //4bit
	AdaptationField            *AdaptationField //optional
	PayloadPointer             uint8
}

//https://en.wikipedia.org/wiki/MPEG_transport_stream
//https://zh.wikipedia.org/wiki/MPEG2-TS
//https://blog.csdn.net/Kayson12345/article/details/81266587
func NewTs(pid uint16, cc map[uint16]uint8, firstTs bool) (ts *TS) {
	ts = &TS{
		SyncByte:                   0x47,
		TransportErrorIndicator:    0x00,
		PayloadUnitStartIndicator:  0x00,
		TransportPriority:          0x00,
		PID:                        pid,
		TransportScramblingControl: 0x00,
		AdaptationFieldExist:       0x01,
		ContinuityCounter:          cc[pid],
		//AdaptationField            :,
		PayloadPointer: 0x00,
	}
	cc[pid] = (cc[pid] + 1) % 0x10
	if firstTs {
		ts.PayloadUnitStartIndicator = 0x01
	}

	//TODO
	switch ts.TransportScramblingControl {
	case 0x00:
	case 0x01:
	case 0x02:
	case 0x03:
	default:
	}

	//TODO
	switch ts.AdaptationFieldExist {
	case 0x00:
	case 0x01:
	case 0x02:
	case 0x03:
	default:
	}

	return ts
}

func (ts *TS) DeMux(pidTable map[uint16]PSI, reader easyio.EasyReader) (err error) {
	b, err := reader.ReadN(4)
	if err != nil {
		return err
	}

	if b[0] != 0x47 {
		return INVALID_DATA_ERROR
	}

	ts.SyncByte = b[0]
	ts.TransportErrorIndicator = (b[1] & 0x80) >> 7
	ts.PayloadUnitStartIndicator = (b[1] & 0x40) >> 6
	ts.TransportPriority = (b[1] & 0x20) >> 5
	ts.PID = uint16(b[1]&0x1f)<<8 + uint16(b[2])
	ts.TransportScramblingControl = (b[3] & 0xc0) >> 6
	ts.AdaptationFieldExist = (b[3] & 0x30) >> 4
	ts.ContinuityCounter = b[3] & 0x0f

	if ts.AdaptationFieldExist == 0x02 || ts.AdaptationFieldExist == 0x03 {
		ts.AdaptationField = NewAdaptationField()
		ts.AdaptationField.Parse(reader)
	}

	if ts.PayloadUnitStartIndicator == 0x01 {
		b, err1 := reader.ReadN(1)
		b, err2 := reader.ReadN(uint32(b[0]))
		if err := easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
			return INVALID_DATA_ERROR
		}
	}

	psi, exist := pidTable[ts.PID]
	if !exist {
		return fmt.Errorf("invalid pid:%d", ts.PID)
	}
	return psi.Parse(reader)
}

func (ts *TS) Mux(psi PSI, firstTSofKeyFrame bool, dts uint64, writer easyio.EasyWriter) (finish bool, err error) {
	var writable uint8 = 188
	var err1, err2, err3, err4 error

	if firstTSofKeyFrame {
		ts.AdaptationFieldExist |= 0x02
		ts.AdaptationField = &AdaptationField{
			AdaptationFieldLength:          0x07,
			RandomAccessIndicator:          0x01,
			PCRFlag:                        0x01,
			ProgramClockReferenceBase:      dts,
			ProgramClockReferenceExtension: 0,
		}
	}

	if ts.PID == AUDIO_PID || ts.PID == VIDEO_PID {
		var headerSize uint8 = 4 //ts header
		if firstTSofKeyFrame {
			headerSize += 8 //adaptation length
		} else {
			ts.AdaptationField = &AdaptationField{}
		}

		if remain := psi.Remain(); remain < int(writable-headerSize) {
			ts.AdaptationFieldExist |= 0x02
			ts.AdaptationField.AdaptationFieldLength = uint8(int(writable-headerSize) - remain - 1)
		}
	}

	b := []byte{
		ts.SyncByte,
		((ts.TransportErrorIndicator << 7) & 0x80) | ((ts.PayloadUnitStartIndicator << 6) & 0x40) | ((ts.TransportPriority << 5) & 0x20) | (uint8(ts.PID>>8) & 0x1f),
		uint8(ts.PID) & 0xff,
		((ts.TransportScramblingControl << 6) & 0xc0) | ((ts.AdaptationFieldExist << 4) & 0x30) | (ts.ContinuityCounter & 0x0f),
	}
	err1 = writer.WriteFull(b)
	writable -= uint8(len(b))

	//AdaptationField
	if (ts.AdaptationFieldExist&0x02) != 0 && ts.AdaptationField != nil {
		var n uint8
		n, err2 = ts.AdaptationField.Marshal(writer)
		writable -= n
	}

	//XXX: better?
	//XXX: pointer is added in psi only? why pes not?
	if (ts.PID == PAT_PID || ts.PID == PMT_PID) && ts.PayloadUnitStartIndicator == 0x01 {
		writer.Write([]byte{ts.PayloadPointer})
		writable -= 1
	}

	n, finish, err3 := psi.Marshal(writer, int(writable))
	writable -= uint8(n)

	err4 = Fill0XFF(writer, int(writable))
	return finish, easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3, err4)
}

func Fill0XFF(writer easyio.EasyWriter, n int) (err error) {
	for i := 0; i < n; i++ {
		err = writer.WriteFull([]byte{0xff})
		if err != nil {
			return
		}
	}
	return nil
}
