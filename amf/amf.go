package amf

import "github.com/SmartBrave/utils/easyio"

type AMF interface {
	Decode(easyio.EasyReader) ([]interface{}, error)
	Encode(easyio.EasyWriter, interface{}) error
}
