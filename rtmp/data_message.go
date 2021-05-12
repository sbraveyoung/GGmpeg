package rtmp

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/SmartBrave/utils/easyio"
)

type DataMessage struct {
	MessageBase
}

func NewDataMessage(mb MessageBase) (dm *DataMessage) {
	return &DataMessage{
		MessageBase: mb,
	}
}

func (dm *DataMessage) Send() (err error) {
	return nil
}

func (dm *DataMessage) Parse() (err error) {
	var array []interface{}
	array, err = dm.amf.Decode(easyio.NewEasyReader(bytes.NewReader(dm.messagePayload)))
	if err != nil {
		return err
	}

	for index, a := range array {
		fmt.Println("index:", index, " a.type:", reflect.TypeOf(a), " a.Value:", reflect.ValueOf(a))
	}
	return nil
}

func (dm *DataMessage) Do() (err error) {
	return nil
}
