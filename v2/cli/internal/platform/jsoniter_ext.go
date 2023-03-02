package platform

import (
	"reflect"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

// InterfaceCodecExtension is used to decode interface fields
// it'll store the type of the values in a wrapper object
type InterfaceCodecExtension struct {
	jsoniter.DummyExtension
}

func NewInterfaceCodecExtension() *InterfaceCodecExtension {
	return &InterfaceCodecExtension{}
}

func (e *InterfaceCodecExtension) DecorateDecoder(typ reflect2.Type, decoder jsoniter.ValDecoder) jsoniter.ValDecoder {
	if typ.Kind() == reflect.Interface {
		return &interfaceCodec{typ: typ, decoder: decoder}
	}
	return decoder
}

const gqlPackage = "encr.dev/v2/cli/internal/platform/gql"

type interfaceCodec struct {
	typ     reflect2.Type
	decoder jsoniter.ValDecoder
}

// Decode decodes an interface value from a iterator
func (codec *interfaceCodec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	// if it's not an objectvalue, we don't need to bother
	if iter.WhatIsNext() != jsoniter.ObjectValue {
		codec.decoder.Decode(ptr, iter)
		return
	}

	// if it is, we try to resolve the pkgPath, type and content
	val := iter.ReadAny()
	typeName := val.Get("__typename").ToString()
	if typeName == "" {
		iter.ReportError("InterfaceCodecExtension", "missing __typename field")
		return
	}

	// try to instantiate the type
	t := reflect2.TypeByPackageName(gqlPackage, typeName)
	if t == nil {
		iter.ReportError("InterfaceCodecExtension", "cannot find type "+typeName+" in package "+gqlPackage)
		return
	}

	// Need to create a pointer to the pointer of the type to be able to be able
	// to replace placeholder values with the actual values
	item := reflect2.PtrTo(reflect2.PtrTo(t)).New()
	val.ToVal(item)
	if err := val.LastError(); err != nil {
		iter.ReportError("decode", err.Error())
		return
	}

	n := reflect.New(codec.typ.Type1())
	n.Elem().Set(reflect.ValueOf(item).Elem().Elem())
	codec.typ.UnsafeSet(ptr, n.UnsafePointer())
}

// IsEmpty checks if a ptr is empty/nil
func (codec *interfaceCodec) IsEmpty(ptr unsafe.Pointer) bool {
	return codec.typ.UnsafeIsNil(ptr)
}
