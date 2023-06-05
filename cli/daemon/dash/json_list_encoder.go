package dash

import (
	"reflect"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

type ListEncoderExtension struct {
	jsoniter.DummyExtension
}

func NewListEncoderExtension() *ListEncoderExtension {
	return &ListEncoderExtension{}
}

func (e *ListEncoderExtension) DecorateEncoder(typ reflect2.Type, encoder jsoniter.ValEncoder) jsoniter.ValEncoder {
	if typ.Kind() == reflect.Slice {
		return &sliceEncoder{typ: typ, encoder: encoder}
	}
	return encoder
}

type sliceEncoder struct {
	typ     reflect2.Type
	encoder jsoniter.ValEncoder
}

func (codec *sliceEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return codec.encoder.IsEmpty(ptr)
}

func (codec *sliceEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	if codec.IsEmpty(ptr) {
		if codec.typ.Type1().Elem().Kind() == reflect.Uint8 {
			stream.WriteString("")
		} else {
			stream.WriteEmptyArray()
		}
		return
	}

	codec.encoder.Encode(ptr, stream)
}
