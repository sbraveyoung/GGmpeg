package libamf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/SmartBrave/utils_sb/easyerrors"
	"github.com/SmartBrave/utils_sb/easyio"
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
	InvalidMarker
)

type amf0 struct{}

var AMF0 amf0

func (amf0) Decode(r easyio.EasyReader) (res []interface{}, err error) {
	var referenceIndex []int
	var i interface{}
	var marker Marker
	for {
		marker, i, err = decodeamf0(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, err
		}
		//NOTE:i should not be set to nil value and typed pointer such as `i=(int*)nil`
		// if i != nil {
		switch marker {
		case ObjectMarker, EcmaArrayMarker, TypedObjectMarker:
			referenceIndex = append(referenceIndex, len(res))
		case ReferenceMarker:
			index := i.(uint16)
			if int(index) < len(referenceIndex) {
				i = res[referenceIndex[index]]
			}
		default:
			//do nothing
		}
		res = append(res, i)
		// }
	}
	return res, nil
}

func decodeamf0(r easyio.EasyReader) (marker Marker, i interface{}, err error) {
	var b []byte
	b, err = r.ReadN(1)
	if err != nil {
		return InvalidMarker, nil, err
	}

	marker = Marker(b[0])
	switch marker {
	case NumberMarker:
		i, err = decodeNumberamf0(r)
	case BooleanMarker:
		i, err = decodeBooleanamf0(r)
	case StringMarker:
		i, err = decodeStringamf0(r)
	case ObjectMarker: //complex types
		i, err = decodeObjectamf0(r)
	case MovieclipMarker: //not supported, do nothing
	case NULLMarker:
		i, err = nil, nil
	case UndefinedMarker: //no futher information is encoded, do nothing
	case ReferenceMarker:
		i, err = decodeReferenceamf0(r)
	case EcmaArrayMarker: //complex types
		i, err = decodeEcmaArrayamf0(r)
	case ObjectEndMarker: //no futher information is encoded, do nothing
	case StrictArrayMarker:
		i, err = decodeStrictArrayamf0(r)
	case DateMarker:
		i, err = decodeDateamf0(r)
	case LongStringMarker:
		i, err = decodeLongStringamf0(r)
	case UnSupportedMarker: //no futher information is encoded, do nothing
	case RecordSetMarker: //not supported, do nothing
	case XMLDocumentMarker:
		i, err = decodeXMLDocumentamf0(r)
	case TypedObjectMarker: //complex types
		//XXX: get className, then?
		var className string
		className, i, err = decodeTypedObjectamf0(r)
		i = map[string]interface{}{
			className: i,
		}
	default:
		return InvalidMarker, i, errors.New("invalid amf0 marker")
	}
	// fmt.Printf("marker:%x, value:%+v\n", marker, i)
	return marker, i, err
}

func decodeNumberamf0(r easyio.EasyReader) (num float64, err error) {
	err = binary.Read(r, binary.BigEndian, &num)
	return num, err
}

func decodeBooleanamf0(r easyio.EasyReader) (boolean bool, err error) {
	err = binary.Read(r, binary.BigEndian, &boolean)
	return boolean, err
}

func decodeStringamf0(r easyio.EasyReader) (str string, err error) {
	var length uint16
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return str, err
	}

	var b []byte
	b, err = readByteamf0(r, uint32(length))
	return string(b), err
}

func decodeLongStringamf0(r easyio.EasyReader) (str string, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return str, err
	}

	var b []byte
	b, err = readByteamf0(r, length)
	return string(b), err
}

//TODO: utf-8 support
func readByteamf0(r easyio.EasyReader, length uint32) (b []byte, err error) {
	b, err = r.ReadN(uint32(length))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func decodeObjectamf0(r easyio.EasyReader) (res map[string]interface{}, err error) {
	var p *pair
	res = make(map[string]interface{})
	for {
		p, err = readPairamf0(r)
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

func decodeTypedObjectamf0(r easyio.EasyReader) (className string, res map[string]interface{}, err error) {
	className, err = decodeStringamf0(r)
	if err != nil {
		return className, res, err
	}

	res, err = decodeObjectamf0(r)
	return className, res, err
}

type pair struct {
	key   string
	value interface{}
}

func readPairamf0(r easyio.EasyReader) (p *pair, err error) {
	p = &pair{}
	p.key, err = decodeStringamf0(r)
	if err != nil {
		return p, err
	}
	if p.key == "" {
		return p, nil
	}

	_, p.value, err = decodeamf0(r)
	if err != nil {
		return p, err
	}
	return p, nil
}

func decodeReferenceamf0(r easyio.EasyReader) (index uint16, err error) {
	err = binary.Read(r, binary.BigEndian, &index)
	return index, err
}

func decodeEcmaArrayamf0(r easyio.EasyReader) (res map[string]interface{}, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return res, err
	}

	res = make(map[string]interface{})
	var i uint32
	var p *pair
	for i = 0; i <= length; i++ { //length: 00 00 09
		p, err = readPairamf0(r)
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

func decodeStrictArrayamf0(r easyio.EasyReader) (res []interface{}, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return res, err
	}

	var i uint32
	var item interface{}
	for i = 0; i < length; i++ {
		_, item, err = decodeamf0(r)
		if err != nil {
			return res, err
		}
		res = append(res, item)
	}
	return res, nil
}

func decodeDateamf0(r easyio.EasyReader) (date time.Time, err error) {
	var timestamp float64
	timestamp, err = decodeNumberamf0(r)
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

func decodeXMLDocumentamf0(r easyio.EasyReader) (xml []byte, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return xml, err
	}

	return readByteamf0(r, length)
}

func (amf0) Encode(w easyio.EasyWriter, obj interface{}) (err error) {
	return encodeamf0(w, obj, true)
}

func encodeamf0(w easyio.EasyWriter, obj interface{}, encodeMarker bool) (err error) {
	if obj == nil {
		binary.Write(w, binary.BigEndian, NULLMarker)
		return
	}

	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		binary.Write(w, binary.BigEndian, UndefinedMarker)
		return
	}
	fmt.Printf("encode, object:%+v\n", obj)

	//NOTE: not support ReferenceMarker yet
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		err = encodeNumberamf0(w, float64(v.Int()), encodeMarker)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		err = encodeNumberamf0(w, float64(v.Uint()), encodeMarker)
	case reflect.Float32, reflect.Float64:
		err = encodeNumberamf0(w, v.Float(), encodeMarker)
	case reflect.Bool:
		err = encodeBooleanamf0(w, v.Bool(), encodeMarker)
	case reflect.String:
		err = encodeStringamf0(w, v.String(), encodeMarker)
	case reflect.Struct: //TODO: has some problem
	//TODO: XMLDocumentMarker
	//TODO: DateMarker
	//TypedObjectMarker
	// binary.Write(w, binary.BigEndian, TypedObjectMarker)
	// err = encodeamf0(w, v.Type().Name())
	// if err != nil {
	// return err
	// }
	// for i := 0; i < v.NumField(); i++ {
	// err = encodeamf0(w, v.Field(i))
	// if err != nil {
	// return err
	// }
	// }
	// binary.Write(w, binary.BigEndian, ObjectEndMarker)
	case reflect.Map: //ObjectMarker
		err = encodeObjectamf0(w, v, encodeMarker)
	case reflect.Ptr, reflect.Interface:
		err = encodeamf0(w, v.Elem(), encodeMarker)
		if err != nil {
			return err
		}
	case reflect.UnsafePointer: //XXX not support yet
		fallthrough
	case reflect.Uintptr: //XXX not support yet
		return errors.New("invalid type")
	case reflect.Slice, reflect.Array: //StrictArrayMarker, EcmaArrayMarker is not supported
		// length := v.Len()
		// binary.Write(w, binary.BigEndian, StrictArrayMarker)
		// binary.Write(w, binary.BigEndian, uint32(length))
		// for i := 0; i < length; i++ {
		// err = encodeamf0(w, v.Index(i))
		// if err != nil {
		// return err
		// }
		// }
		// binary.Write(w, binary.BigEndian, ObjectEndMarker)
	default:
		//TODO
	}
	return err
}

func encodeNumberamf0(w easyio.EasyWriter, num float64, encodeMarker bool) (err error) {
	var err1, err2 error
	if encodeMarker {
		err1 = binary.Write(w, binary.BigEndian, NumberMarker)
	}
	err2 = binary.Write(w, binary.BigEndian, float64(num))
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}

func encodeBooleanamf0(w easyio.EasyWriter, boolean bool, encodeMarker bool) (err error) {
	var err1, err2 error
	if encodeMarker {
		err1 = binary.Write(w, binary.BigEndian, BooleanMarker)
	}
	err2 = binary.Write(w, binary.BigEndian, boolean)
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}

func encodeStringamf0(w easyio.EasyWriter, str string, encodeMarker bool) (err error) {
	var err1, err2, err3 error
	if encodeMarker {
		if len(str) <= 0xffff {
			err1 = binary.Write(w, binary.BigEndian, StringMarker)
		} else {
			err1 = binary.Write(w, binary.BigEndian, LongStringMarker)
		}
	}
	if len(str) <= 0xffff {
		err2 = binary.Write(w, binary.BigEndian, uint16(len(str)))
	} else {
		err2 = binary.Write(w, binary.BigEndian, uint32(len(str)))
	}
	_, err3 = w.Write([]byte(str))
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
}

func encodeObjectamf0(w easyio.EasyWriter, obj reflect.Value, encodeMarker bool) (err error) {
	var err1, err2, err3 error
	if encodeMarker {
		err1 = binary.Write(w, binary.BigEndian, ObjectMarker)
	}
	iter := obj.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() != reflect.String {
			return errors.New("invalid type")
		}
		err = encodeamf0(w, key.String(), false)
		if err != nil {
			return err
		}
		value := iter.Value()
		err = encodeamf0(w, value.Interface(), true)
		if err != nil {
			return err
		}
	}
	_, err2 = w.Write([]byte{0x00, 0x00})
	err3 = binary.Write(w, binary.BigEndian, ObjectEndMarker)
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
}
