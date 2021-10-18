package mpeg

//https://ocw.unican.es/pluginfile.php/171/course/section/78/iso13818-1.pdf

import (
	"errors"

	"github.com/SmartBrave/Athena/easyerrors"
	"github.com/SmartBrave/Athena/easyio"
)

type PCR struct {
	Low  int64
	High int16
}

type TSAdaptationField struct {
	AdaptationFieldLength        uint8 //8bit
	DiscontinuityIndicator       bool  //1bit
	RandomAccessIndicator        bool  //1bit
	ElementaryStreamPriority     bool  //1bit
	PCRFlag                      bool  //1bit
	OPCRFlag                     bool  //1bit
	SplicingPointFlag            bool  //1bit
	TransportPrivateDataFlag     bool  //1bit
	AdaptationFieldExtensionFlag bool  //1bit
	PCRField                     *PCR  //optional
	OPCRField                    *PCR  //optional
	SpliceCountdown              int8  //optional
}

type TS struct {
	SyncByte                   byte               //8bit
	TransportErrorIndicator    bool               //1bit
	PayloadUnitStartIndicator  bool               //1bit
	TransportPriority          bool               //1bit
	PID                        uint16             //13bit
	TransportScramblingControl byte               //2bit
	AdaptationFieldExist       byte               //2bit
	ContinuityCounter          byte               //4bit
	TsAdaptationField          *TSAdaptationField //optional
}

//https://zh.wikipedia.org/wiki/MPEG2-TS
//https://blog.csdn.net/Kayson12345/article/details/81266587
func NewTs(r easyio.EasyReader) (ts *TS, err error) {
	b, err := r.ReadN(4)
	if err != nil {
		return nil, err
	}

	if b[0] != 0x47 {
		return nil, errors.New("invalid data")
	}
	ts = &TS{
		SyncByte:                   b[0],
		TransportErrorIndicator:    b[1]&0x80 == 0x80,
		PayloadUnitStartIndicator:  b[1]&0x40 == 0x40,
		TransportPriority:          b[1]&0x20 == 0x20,
		PID:                        uint16((b[1]&0x1f)<<8) + uint16(b[2]),
		TransportScramblingControl: (b[3] & 0xc0) >> 6,
		AdaptationFieldExist:       (b[3] & 0x30) >> 4,
		ContinuityCounter:          b[3] & 0x0f,
	}
	if ts.AdaptationFieldExist == 0x02 || ts.AdaptationFieldExist == 0x03 {
		var err1, err2 error
		b, err1 = r.ReadN(1)
		length := uint8(b[0])
		b, err2 = r.ReadN(uint32(length))
		if err = easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2); err != nil {
			return nil, errors.New("invalid data")
		}
		if len(b) >= 1 {
			ts.TsAdaptationField = &TSAdaptationField{
				AdaptationFieldLength:        length,
				DiscontinuityIndicator:       b[0]&0x80 == 0x80,
				RandomAccessIndicator:        b[0]&0x40 == 0x40,
				ElementaryStreamPriority:     b[0]&0x20 == 0x20,
				PCRFlag:                      b[0]&0x10 == 0x10,
				OPCRFlag:                     b[0]&0x08 == 0x08,
				SplicingPointFlag:            b[0]&0x04 == 0x04,
				TransportPrivateDataFlag:     b[0]&0x02 == 0x02,
				AdaptationFieldExtensionFlag: b[0]&0x01 == 0x01,
			}
		}

		if ts.TsAdaptationField.PCRFlag && len(b) >= 7 {
			ts.TsAdaptationField.PCRField = &PCR{
				Low:  int64(b[1]<<32) + int64(b[2]<<24) + int64(b[3]<<16) + int64(b[4]<<8) + int64(b[5]&0x80),
				High: int16((b[5]&0x01)<<8) + int16(b[6]),
			}
		}
		if ts.TsAdaptationField.OPCRFlag && len(b) >= 13 {
			ts.TsAdaptationField.OPCRField = &PCR{
				Low:  int64(b[7]<<32) + int64(b[8]<<24) + int64(b[9]<<16) + int64(b[10]<<8) + int64(b[11]&0x80),
				High: int16((b[11]&0x01)<<8) + int16(b[12]),
			}
		}
		if len(b) >= 14 {
			ts.TsAdaptationField.SpliceCountdown = int8(b[13])
		}
	}
	return ts, nil
}
