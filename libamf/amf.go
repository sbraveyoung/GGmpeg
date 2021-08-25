package libamf

import "github.com/SmartBrave/utils_sb/easyio"

type AMF interface {
	Decode(easyio.EasyReader) ([]interface{}, error)
	Encode(easyio.EasyWriter, interface{}) error
}
