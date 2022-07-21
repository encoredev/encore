package serde

import (
	"bytes"
	"reflect"

	jsoniter "github.com/json-iterator/go"
)

type JSONSerializer struct {
	buffer        *bytes.Buffer
	stream        *jsoniter.Stream
	writtenFields int
}

func SerializeJSONFunc(cfg jsoniter.API, fn func(serializer *JSONSerializer)) ([]byte, error) {
	s := &JSONSerializer{}
	s.buffer = new(bytes.Buffer)
	s.stream = jsoniter.NewStream(cfg, s.buffer, 1024)
	s.stream.WriteObjectStart()
	fn(s)
	s.stream.WriteObjectEnd()
	err := s.stream.Flush()
	if err != nil {
		return nil, err
	}
	return s.buffer.Bytes(), s.stream.Error
}

func (s *JSONSerializer) WriteField(name string, val any, omitEmpty bool) {
	if omitEmpty && reflect.ValueOf(val).IsZero() {
		return
	}
	if s.writtenFields > 0 {
		s.stream.WriteMore()
	}
	s.stream.WriteObjectField(name)
	s.stream.WriteVal(val)
	s.writtenFields++
}

func SerializeInputs(json jsoniter.API, inputs ...any) ([][]byte, error) {
	outputs := make([][]byte, len(inputs))
	for i, input := range inputs {
		out, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		outputs[i] = out
	}
	return outputs, nil
}
