package errmarshalling

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

func init() {
	marshaller := &jsonMarshaller{
		unmarshal: fallbackUnmarshal,
		decoder: func(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
			err := fallbackUnmarshal(iter)
			fmt.Println("here with ", err)
		},
	}
	serdeByName["error"] = marshaller
}

func fallbackUnmarshal(itr *jsoniter.Iterator) error {
	var errMsg string
	var wrapped error
	var wrappedList []error

	itr.ReadObjectCB(func(itr *jsoniter.Iterator, field string) bool {
		switch field {
		case MessageKey:
			errMsg = itr.ReadString()
		case WrappedKey:
			switch itr.WhatIsNext() {
			case jsoniter.ArrayValue:
				itr.ReadArrayCB(func(itr *jsoniter.Iterator) bool {
					wrappedList = append(wrappedList, UnmarshalError(itr))
					return true
				})
			case jsoniter.ObjectValue:
				wrapped = UnmarshalError(itr)
			default:
				itr.ReportError("unmarshal", "expected array or object")
				itr.Skip()
			}
		default:
			itr.Skip()
		}
		return true
	})

	switch {
	case len(wrappedList) > 0:
		return &fallbackMultiWrapErr{
			msg:     errMsg,
			wrapped: wrappedList,
		}
	case wrapped != nil:
		return &fallbackSingleWrapErr{
			msg:     errMsg,
			wrapped: wrapped,
		}
	default:
		return errors.New(errMsg)
	}
}

func createFallbackEncoder(typ reflect2.Type) jsoniter.ValEncoder {
	return &jsonMarshaller{
		encoder: func(ptr unsafe.Pointer, stream *jsoniter.Stream) {
			pointerToVal := typ.PackEFace(ptr)                        // *[error type]
			iface := reflect.ValueOf(pointerToVal).Elem().Interface() // [error type]
			err := iface.(error)                                      // error

			// check if we have a custom marshaller for this type
			// if we do then use it.
			//
			// this path can occur if the field is strongly typed as `error`
			_, found := serdeByType[reflect2.TypeOf(iface)]
			if found {
				// this will go via the custom marshaller now
				stream.WriteVal(iface)
				return
			}

			stream.WriteObjectStart()

			stream.WriteObjectField(TypeKey)
			stream.WriteString("error")
			stream.WriteMore()

			stream.WriteObjectField(MessageKey)
			stream.WriteString(err.Error())

			switch err := err.(type) {
			case interface{ Unwrap() error }:
				wrapped := err.Unwrap()
				if wrapped != nil {
					stream.WriteMore()
					stream.WriteObjectField(WrappedKey)
					stream.WriteVal(wrapped)
				}

			case interface{ Unwrap() []error }:
				wrapped := err.Unwrap()
				if len(wrapped) > 0 {
					stream.WriteMore()
					stream.WriteObjectField(WrappedKey)
					stream.WriteVal(wrapped)
				}
			}

			stream.WriteObjectEnd()
		},
	}
}

// fallbackSingleWrapErr is a fallback error type that wraps a single error.
type fallbackSingleWrapErr struct {
	msg     string
	wrapped error
}

var _ error = (*fallbackSingleWrapErr)(nil)

func (f *fallbackSingleWrapErr) Error() string {
	return f.msg
}

func (f *fallbackSingleWrapErr) Unwrap() error {
	return f.wrapped
}

// fallbackMultiWrapErr is a fallback error type that wraps multiple errors.
type fallbackMultiWrapErr struct {
	msg     string
	wrapped []error
}

var _ error = (*fallbackMultiWrapErr)(nil)

func (f *fallbackMultiWrapErr) Error() string {
	return f.msg
}

func (f *fallbackMultiWrapErr) Unwrap() []error {
	return f.wrapped
}
