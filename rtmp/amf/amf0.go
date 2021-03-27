package amf

import (
	"encoding/binary"
	stdio "io"
	"time"

	"github.com/SmartBrave/utils/io"
)

type Marker uint8

const (
	NumberMarker Marker = 0x00 + iota
	BooleanMarker
	StringMarker
	ObjectMarker    //complex types
	MovieclipMarker //reserved, not supported
	NULLMarker
	UndefinedMarker
	ReferenceMarker
	EcmaArrayMarker //complex types
	ObjectEndMarker
	StrictArrayMarker
	DateMarker
	LongStringMarker
	UnSupportedMarker
	RecordSetMarker //reserved, not supported
	XMLDocumentMarker
	TypedObjectMarker //complex types
)

type amf0 struct{}

var AMF0 amf0

func (amf0) Decode(r io.Reader) (res []interface{}, err error) {
	var b []byte
	var referenceIndex []int
	for {
		var i interface{}
		b, err = r.ReadN(1)
		if err == stdio.EOF {
			return res, nil
		}
		if err != nil {
			return nil, err
		}

		switch Marker(b[0]) {
		case NumberMarker:
			i, err = decodeNumber(r)
		case BooleanMarker:
			i, err = decodeBoolean(r)
		case StringMarker:
			i, err = decodeString(r)
		case ObjectMarker: //complex types
			//TODO
		case MovieclipMarker: //not supported, do nothing
		case NULLMarker: //no futher information is encoded, do nothing
		case UndefinedMarker: //no futher information is encoded, do nothing
		case ReferenceMarker:
			var index uint16
			index, err = decodeReference(r)
			if int(index) < len(referenceIndex) {
				i = res[referenceIndex[index]]
			}
		case EcmaArrayMarker: //complex types
			//TODO
		case ObjectEndMarker:
			//TODO
		case StrictArrayMarker:
			i, err = decodeStrictArray(r)
		case DateMarker:
			i, err = decodeDate(r)
		case LongStringMarker:
			i, err = decodeLongString(r)
		case UnSupportedMarker: //no futher information is encoded, do nothing
		case RecordSetMarker: //not supported, do nothing
		case XMLDocumentMarker:
			i, err = decodeXMLDocument(r)
		case TypedObjectMarker: //complex types
			//TODO
		default:
		}

		if err != nil {
			return res, err
		}

		if i != nil {
			//NOTE:i should not be set to nil value and typed pointer
			res = append(res, i)
			switch Marker(b[0]) {
			case ObjectMarker, EcmaArrayMarker, TypedObjectMarker:
				referenceIndex = append(referenceIndex, len(res)-1)
			default:
				//do nothing
			}
		}
	}
}

func decodeNumber(r io.Reader) (num float64, err error) {
	err = binary.Read(r, binary.BigEndian, &num)
	return num, err
}

func decodeBoolean(r io.Reader) (boolean bool, err error) {
	err = binary.Read(r, binary.BigEndian, &boolean)
	return boolean, err
}

func decodeString(r io.Reader) (str string, err error) {
	var length uint16
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return str, err
	}

	var b []byte
	b, err = readByte(r, int(length))
	return string(b), err
}

func decodeLongString(r io.Reader) (str string, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return str, err
	}

	var b []byte
	b, err = readByte(r, int(length))
	return string(b), err
}

//TODO: support utf-8
func readByte(r io.Reader, length int) (b []byte, err error) {
	b, err = r.ReadN(int(length))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func decodeReference(r io.Reader) (index uint16, err error) {
	err = binary.Read(r, binary.BigEndian, &index)
	return index, err
}

func decodeStrictArray(r io.Reader) (res []interface{}, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return res, err
	}

	var i uint32
	var item interface{}
	for i = 0; i < length; i++ {
		item, err = AMF0.Decode(r)
		if err != nil {
			return res, err
		}
		res = append(res, item)
	}
	return res, nil
}

func decodeDate(r io.Reader) (date time.Time, err error) {
	var timestamp float64
	timestamp, err = decodeNumber(r)
	if err != nil {
		return time.Unix(0, 0), err
	}

	var timeZone int16
	err = binary.Read(r, binary.BigEndian, &timeZone)
	if err != nil {
		return time.Unix(0, 0), err
	}

	return time.Unix(int64(timestamp), 0), nil
}

func decodeXMLDocument(r io.Reader) (xml []byte, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return xml, err
	}

	return readByte(r, int(length))
}
