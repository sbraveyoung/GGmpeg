package amf

import (
	"encoding/binary"
	"errors"
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
	return decode(r)
}

func decode(r io.Reader) (res []interface{}, err error) {
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
			i, err = decodeObject(r)
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
			i, err = decodeEcmaArray(r)
		case ObjectEndMarker: //no futher information is encoded, do nothing
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
			var className string
			className, i, err = decodeTypedObject(r)
			i = map[string]interface{}{
				className: i,
			}
		default:
			return res, errors.New("invalid amf0 marker")
		}

		if err != nil {
			return res, err
		}

		//NOTE:i should not be set to nil value and typed pointer such as `i=(int*)nil`
		if i != nil {
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

func decodeObject(r io.Reader) (res map[string]interface{}, err error) {
	var p *pair
	for {
		p, err = readPair(r)
		if err != nil {
			return res, err
		}
		if p.key == "" {
			break
		}
		res[p.key] = p.value
	}
	return res, nil
}

func decodeTypedObject(r io.Reader) (className string, res map[string]interface{}, err error) {
	className, err = decodeString(r)
	if err != nil {
		return className, res, err
	}

	res, err = decodeObject(r)
	return className, res, err
}

type pair struct {
	key   string
	value interface{}
}

func readPair(r io.Reader) (p *pair, err error) {
	p = &pair{}
	p.key, err = decodeString(r)
	if err != nil {
		return p, err
	}
	if p.key == "" {
		return p, nil
	}

	p.value, err = decode(r)
	if err != nil {
		return p, err
	}
	return p, nil
}

func decodeReference(r io.Reader) (index uint16, err error) {
	err = binary.Read(r, binary.BigEndian, &index)
	return index, err
}

func decodeEcmaArray(r io.Reader) (res map[string]interface{}, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)

	var i uint32
	var p *pair
	for i = 0; i < length; i++ {
		p, err = readPair(r)
		if err != nil {
			return res, err
		}
		if p.key == "" {
			break
		}
		res[p.key] = p.value
	}
	return res, nil
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
		item, err = decode(r)
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

	return time.Unix(0, int64(timestamp)*1e6), nil
}

func decodeXMLDocument(r io.Reader) (xml []byte, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return xml, err
	}

	return readByte(r, int(length))
}
