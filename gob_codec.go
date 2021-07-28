package queue

import (
	"bytes"
	"encoding/gob"
	"reflect"

	"github.com/DoNewsCode/core/contract"
)

var _ contract.Codec = gobCodec{}

type gobCodec struct{}

// Marshal serializes the message to bytes
func (p gobCodec) Marshal(message interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(message); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal reverses the bytes to message
func (p gobCodec) Unmarshal(data []byte, message interface{}) error {
	buf := bytes.NewBuffer(data)
	if rvalue, ok := message.(reflect.Value); ok {
		return gob.NewDecoder(buf).DecodeValue(rvalue)
	}
	return gob.NewDecoder(buf).Decode(message)
}
