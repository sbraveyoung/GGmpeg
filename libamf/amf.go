package libamf

import "github.com/SmartBrave/Athena/easyio"

type AMF interface {
	Decode(easyio.EasyReader) ([]interface{}, error)
	Encode(easyio.EasyWriter, interface{}) error
}
