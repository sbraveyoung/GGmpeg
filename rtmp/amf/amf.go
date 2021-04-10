package amf

import "github.com/SmartBrave/utils/easyio"

type AMF interface {
	Decode(easyio.Reader) ([]interface{}, error)
	Encode(easyio.Writer, interface{}) error
}
