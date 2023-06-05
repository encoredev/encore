package errmarshalling

import (
	"fmt"
	"reflect"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

var (
	// jsonMarshaller by reflect2 type (we use this for speed in the extension)
	serdeByType = map[reflect2.Type]*jsonMarshaller{}

	// jsonMarshaller by name (we use this for use in Unmarshal)
	serdeByName = map[string]*jsonMarshaller{}

	// errType is the reflect2 type of the error interface
	errType = reflect2.Type2(reflect.TypeOf((*error)(nil)).Elem())
)

const (
	// TypeKey is the key used to identify the type of an error
	// used to unmarshal the correct error type
	TypeKey = "@type"

	// MessageKey is the key used to identify the message of an error
	// we use a common name, which allows the fallback unmarshaler to work
	MessageKey = "msg"

	// WrappedKey is the key used to identify the wrapped error(s) of an error
	// it can be either a single object or an array of objects
	// again we use a common name, which allows the fallback unmarshaler to work
	WrappedKey = "wraps"
)

// RegisterErrorMarshaller registers a custom marshaller for a specific error type
//
// The error type has to be used as a pointer, and must be registered before any
// calls to [Marshal] or [Unmarshal] are made.
//
// The encoder and decoder functions are responsible for encoding and decoding
// the error object.
//
// Encoder will be passed a *jsoniter.Stream which is already in object mode.
// so fields can be written directly to it without needing to call WriteObjectStart or WriteObjectEnd.
//
// Decoder will be passed a *jsoniter.Iterator before the object has been read.
// allowing calls to ReadObjectCB to read the fields of the object.
func RegisterErrorMarshaller[T error](encoder func(T, *jsoniter.Stream), decoder func(T, *jsoniter.Iterator)) {
	var zero T
	typ := reflect2.TypeOf(zero)
	if !typ.LikePtr() {
		panic(fmt.Errorf("error type %s must be a pointer", typ.String()))
	}
	baseType := reflect2.TypeOfPtr(zero).Elem()
	typeName := baseType.Type1().PkgPath() + "." + baseType.Type1().Name()

	marshaller := &jsonMarshaller{
		unmarshal: func(itr *jsoniter.Iterator) error {
			zero := baseType.New()
			itr.ReadVal(&zero)
			return zero.(error)
		},
		encoder: func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
			stream.WriteObjectStart()

			// Write the type name
			stream.WriteObjectField(TypeKey)
			stream.WriteString(typeName)
			stream.WriteMore()

			// Hand over to the encoder provided
			encoder(*(*T)(ptr), stream)

			stream.WriteObjectEnd()
		},
		decoder: func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
			// Create a new instance of the error object
			obj := baseType.UnsafeNew()

			// let the passed in decoder do it's job
			decoder(baseType.PackEFace(obj).(T), iter)

			// set the pointer to the new object that we just unmarhsalled
			baseType.UnsafeSet(ptr, obj)
		},
	}

	serdeByName[typeName] = marshaller
	serdeByType[typ] = marshaller
}

// jsonExtension is a jsoniter extension that allows us to register custom
// error encoder and decoders using [RegisterErrorMarshaller]
type jsonExtension struct {
	jsoniter.DummyExtension
}

func (extension *jsonExtension) CreateDecoder(typ reflect2.Type) jsoniter.ValDecoder {
	// If somebody is trying to unmarshal an error directly
	// we know how to do that
	if typ == errType {
		return &jsonMarshaller{decoder: func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
			err := UnmarshalError(iter)
			errType.UnsafeSet(ptr, unsafe.Pointer(&err))
		}}
	}

	// jsoniter is asking if we know how to decode this value
	// we index by the base type, but we get a pointer type here - hence the "PtrTo" call.
	if f, ok := serdeByType[reflect2.PtrTo(typ)]; ok {
		return f
	}
	return nil
}

func (extension *jsonExtension) CreateEncoder(typ reflect2.Type) jsoniter.ValEncoder {
	// jsoniter is asking if we know how to encode this value
	// we'll be passed a pointer type here, so we can index directly
	if f, ok := serdeByType[typ]; ok {
		return f
	}

	// However if we don't have a marshaller for this type, we can check if it implements the error interface
	// and if it does, we can create a fallback marshaller for it.
	if typ.Implements(errType) {
		return createFallbackEncoder(typ)
	}

	return nil
}

// jsonMarshaller is a simple struct which implmenets jsoniter's ValEncoder and ValDecoder interfaces
// allowing us to use it as a single object without having to create mulitple objects
type jsonMarshaller struct {
	// unmarshal takes an iterator, creates a new version of the error object, and returns it
	unmarshal func(itr *jsoniter.Iterator) error
	encoder   jsoniter.EncoderFunc
	decoder   jsoniter.DecoderFunc
}

var (
	_ jsoniter.ValEncoder = (*jsonMarshaller)(nil)
	_ jsoniter.ValDecoder = (*jsonMarshaller)(nil)
)

func (j *jsonMarshaller) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	j.encoder(ptr, stream)
}

func (j *jsonMarshaller) IsEmpty(ptr unsafe.Pointer) bool {
	return false
}

func (j *jsonMarshaller) Decode(value unsafe.Pointer, iter *jsoniter.Iterator) {
	j.decoder(value, iter)
}
