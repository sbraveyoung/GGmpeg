package mpeg

import "github.com/SmartBrave/Athena/easyio"

type PES_TYPE uint8

const (
	I PES_TYPE = iota
	P
	B
)

type PES struct {
	StreamType uint8 //8bit, iso13818-1.pdf Table 2-29
	// Reserved1     uint8  //3bit
	ElementaryPID uint16 //13bit
	// Reserved2     uint8  //4bit
	ESInfoLength uint16 //12bit
	//descriptor
}

func NewPES(streamType uint8, elementaryPID, esInfoLength uint16) (pes *PES) {
	return &PES{
		StreamType:    streamType,
		ElementaryPID: elementaryPID,
		ESInfoLength:  esInfoLength,
	}
}

func (pes *PES) Parse(reader easyio.EasyReader) (err error) {
	return nil
}
