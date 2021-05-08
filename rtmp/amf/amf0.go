package amf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	// "fmt"

	"github.com/SmartBrave/utils/easyerrors"
	"github.com/SmartBrave/utils/easyio"
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

type AMF0 struct{}

func (AMF0) Decode(r easyio.EasyReader) (res []interface{}, err error) {
	var referenceIndex []int
	var i interface{}
	var marker Marker
	for {
		marker, i, err = decodeAMF0(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, err
		}
		//NOTE:i should not be set to nil value and typed pointer such as `i=(int*)nil`
		if i != nil {
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
		}
	}
	return res, nil
}

func decodeAMF0(r easyio.EasyReader) (marker Marker, i interface{}, err error) {
	var b []byte
	b, err = r.ReadN(1)
	if err != nil {
		return InvalidMarker, nil, err
	}

	marker = Marker(b[0])
	switch marker {
	case NumberMarker:
		i, err = decodeNumberAMF0(r)
	case BooleanMarker:
		i, err = decodeBooleanAMF0(r)
	case StringMarker:
		i, err = decodeStringAMF0(r)
	case ObjectMarker: //complex types
		i, err = decodeObjectAMF0(r)
	case MovieclipMarker: //not supported, do nothing
	case NULLMarker: //no futher information is encoded, do nothing
	case UndefinedMarker: //no futher information is encoded, do nothing
	case ReferenceMarker:
		i, err = decodeReferenceAMF0(r)
	case EcmaArrayMarker: //complex types
		i, err = decodeEcmaArrayAMF0(r)
	case ObjectEndMarker: //no futher information is encoded, do nothing
	case StrictArrayMarker:
		i, err = decodeStrictArrayAMF0(r)
	case DateMarker:
		i, err = decodeDateAMF0(r)
	case LongStringMarker:
		i, err = decodeLongStringAMF0(r)
	case UnSupportedMarker: //no futher information is encoded, do nothing
	case RecordSetMarker: //not supported, do nothing
	case XMLDocumentMarker:
		i, err = decodeXMLDocumentAMF0(r)
	case TypedObjectMarker: //complex types
		//XXX: get className, then?
		var className string
		className, i, err = decodeTypedObjectAMF0(r)
		i = map[string]interface{}{
			className: i,
		}
	default:
		return InvalidMarker, i, errors.New("invalid amf0 marker")
	}
	// fmt.Printf("marker:%x, value:%+v\n", marker, i)
	return marker, i, err
}

func decodeNumberAMF0(r easyio.EasyReader) (num float64, err error) {
	err = binary.Read(r, binary.BigEndian, &num)
	return num, err
}

func decodeBooleanAMF0(r easyio.EasyReader) (boolean bool, err error) {
	err = binary.Read(r, binary.BigEndian, &boolean)
	return boolean, err
}

func decodeStringAMF0(r easyio.EasyReader) (str string, err error) {
	var length uint16
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return str, err
	}

	var b []byte
	b, err = readByteAMF0(r, int(length))
	return string(b), err
}

func decodeLongStringAMF0(r easyio.EasyReader) (str string, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return str, err
	}

	var b []byte
	b, err = readByteAMF0(r, int(length))
	return string(b), err
}

//TODO: utf-8 support
func readByteAMF0(r easyio.EasyReader, length int) (b []byte, err error) {
	b, err = r.ReadN(int(length))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func decodeObjectAMF0(r easyio.EasyReader) (res map[string]interface{}, err error) {
	var p *pair
	res = make(map[string]interface{})
	for {
		p, err = readPairAMF0(r)
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

func decodeTypedObjectAMF0(r easyio.EasyReader) (className string, res map[string]interface{}, err error) {
	className, err = decodeStringAMF0(r)
	if err != nil {
		return className, res, err
	}

	res, err = decodeObjectAMF0(r)
	return className, res, err
}

type pair struct {
	key   string
	value interface{}
}

func readPairAMF0(r easyio.EasyReader) (p *pair, err error) {
	p = &pair{}
	p.key, err = decodeStringAMF0(r)
	if err != nil {
		return p, err
	}
	// if p.key == "" {
	// return p, nil
	// }

	_, p.value, err = decodeAMF0(r)
	if err != nil {
		return p, err
	}
	return p, nil
}

func decodeReferenceAMF0(r easyio.EasyReader) (index uint16, err error) {
	err = binary.Read(r, binary.BigEndian, &index)
	return index, err
}

func decodeEcmaArrayAMF0(r easyio.EasyReader) (res map[string]interface{}, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return res, err
	}

	var i uint32
	var p *pair
	for i = 0; i < length; i++ {
		p, err = readPairAMF0(r)
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

func decodeStrictArrayAMF0(r easyio.EasyReader) (res []interface{}, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return res, err
	}

	var i uint32
	var item interface{}
	for i = 0; i < length; i++ {
		_, item, err = decodeAMF0(r)
		if err != nil {
			return res, err
		}
		res = append(res, item)
	}
	return res, nil
}

func decodeDateAMF0(r easyio.EasyReader) (date time.Time, err error) {
	var timestamp float64
	timestamp, err = decodeNumberAMF0(r)
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

func decodeXMLDocumentAMF0(r easyio.EasyReader) (xml []byte, err error) {
	var length uint32
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return xml, err
	}

	return readByteAMF0(r, int(length))
}

func (AMF0) Encode(w easyio.EasyWriter, obj interface{}) (err error) {
	return encodeAMF0(w, obj, true)
}

func encodeAMF0(w easyio.EasyWriter, obj interface{}, encodeMarker bool) (err error) {
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
		err = encodeNumberAMF0(w, float64(v.Int()), encodeMarker)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		err = encodeNumberAMF0(w, float64(v.Uint()), encodeMarker)
	case reflect.Float32, reflect.Float64:
		err = encodeNumberAMF0(w, v.Float(), encodeMarker)
	case reflect.Bool:
		err = encodeBooleanAMF0(w, v.Bool(), encodeMarker)
	case reflect.String:
		err = encodeStringAMF0(w, v.String(), encodeMarker)
	case reflect.Struct: //TODO: has some problem
	//TODO: XMLDocumentMarker
	//TODO: DateMarker
	//TypedObjectMarker
	// binary.Write(w, binary.BigEndian, TypedObjectMarker)
	// err = encodeAMF0(w, v.Type().Name())
	// if err != nil {
	// return err
	// }
	// for i := 0; i < v.NumField(); i++ {
	// err = encodeAMF0(w, v.Field(i))
	// if err != nil {
	// return err
	// }
	// }
	// binary.Write(w, binary.BigEndian, ObjectEndMarker)
	case reflect.Map: //ObjectMarker
		err = encodeObjectAMF0(w, v, encodeMarker)
	case reflect.Ptr, reflect.Interface:
		err = encodeAMF0(w, v.Elem(), encodeMarker)
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
		// err = encodeAMF0(w, v.Index(i))
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

func encodeNumberAMF0(w easyio.EasyWriter, num float64, encodeMarker bool) (err error) {
	var err1, err2 error
	if encodeMarker {
		err1 = binary.Write(w, binary.BigEndian, NumberMarker)
	}
	err2 = binary.Write(w, binary.BigEndian, float64(num))
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}

func encodeBooleanAMF0(w easyio.EasyWriter, boolean bool, encodeMarker bool) (err error) {
	var err1, err2 error
	if encodeMarker {
		err1 = binary.Write(w, binary.BigEndian, BooleanMarker)
	}
	err2 = binary.Write(w, binary.BigEndian, boolean)
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2)
}

func encodeStringAMF0(w easyio.EasyWriter, str string, encodeMarker bool) (err error) {
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

func encodeObjectAMF0(w easyio.EasyWriter, obj reflect.Value, encodeMarker bool) (err error) {
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
		err = encodeAMF0(w, key.String(), false)
		if err != nil {
			return err
		}
		value := iter.Value()
		err = encodeAMF0(w, value.Interface(), true)
		if err != nil {
			return err
		}
	}
	_, err2 = w.Write([]byte{0x00, 0x00})
	err3 = binary.Write(w, binary.BigEndian, ObjectEndMarker)
	return easyerrors.HandleMultiError(easyerrors.Simple(), err1, err2, err3)
}
