package errmarshalling

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

var json = JsonAPI()

// JsonAPI returns a jsoniter.API instance that can be used to marshal
// errors
//
// It is only exported for use in tests!
func JsonAPI() jsoniter.API {
	api := jsoniter.Config{
		EscapeHTML:             false,
		IndentionStep:          2,
		SortMapKeys:            true,
		ValidateJsonRawMessage: true,
		TagKey:                 "err_json", // ignore `json` tags (as we use them to hide stuff sometimes)
	}.Froze()

	// Register the JSON extension to allow us to register custom marshallers
	// For our error types
	api.RegisterExtension(&jsonExtension{})

	return api
}

// Marshal will marshal the given error into a JSON byte slice
//
// If the error is nil, the return value will be the JSON null value.
//
// If an error or panic occurs while marshalling the error,
// a generic error will be marshalled instead
func Marshal(err error) (rtn []byte) {
	// Recover from any panics that may occur while marshalling and
	// return a generic error message around the panic
	defer func() {
		if r := recover(); r != nil {
			// because we don't know the cause of the panic, let's write
			// the bytes manually to avoid any further panics if it's being caused by
			// jsoniter
			rtn = []byte(
				fmt.Sprintf(
					"{ \"%s\": \"error\", \"%s\": \"%s\"}",
					TypeKey, MessageKey,
					strconv.Quote(fmt.Sprintf("panic occured while marshalling error: %v", r)),
				),
			)
		}
	}()

	// No error, nothing to marshal
	if err == nil {
		return []byte{'n', 'u', 'l', 'l'}
	}

	// Try to marshal the error using the registered marshallers
	wf, err := json.Marshal(err)
	if err == nil {
		return wf
	}

	// Otherwise fall back and marshal the error as a super generic error
	var buf bytes.Buffer
	stream := json.BorrowStream(&buf)
	defer json.ReturnStream(stream)

	stream.WriteObjectStart()
	stream.WriteObjectField(TypeKey)
	stream.WriteString("error")
	stream.WriteMore()
	stream.WriteObjectField(MessageKey)
	stream.WriteString(err.Error())
	stream.WriteObjectEnd()
	_ = stream.Flush()

	return buf.Bytes()
}

// Unmarshal attempts to unmarshal an error from the given bytes,
//
// On success, the unmarshalled error will be returned, and the error will be nil
// On failure, unmarshalled will be nil and the error will be non-nil
func Unmarshal(bytes []byte) (unmarshalled error, err error) {
	// Read the type name lazily (we will only read enough bytes to get the type name)
	typeName := json.Get(bytes, TypeKey).ToString()

	marshallers, found := serdeByName[typeName]
	if !found {
		return nil, fmt.Errorf("error type %s not registered", typeName)
	}

	itr := json.BorrowIterator(bytes)
	defer json.ReturnIterator(itr)

	unmarshalledError := marshallers.unmarshal(itr)

	return unmarshalledError, itr.Error
}

// UnmarshalError attempts to unmarshal an error from the given iterator,
// returning the unmarshalled error if successful, or nil if not.
func UnmarshalError(itr *jsoniter.Iterator) (unmarshalledError error) {
	unmarshalled, err := Unmarshal(itr.SkipAndReturnBytes())
	if err != nil {
		itr.ReportError("unmarshal", err.Error())
		return nil
	} else {
		return unmarshalled
	}
}

// TryWriteValue attempts to marshal the given value into JSON, returning the
// marshalled bytes if successful, or false if not.
func TryWriteValue(stream *jsoniter.Stream, fieldName string, value any) error {
	if value == nil {
		stream.WriteNil()
		return nil
	}

	// Create a sub-stream to write the value into
	// which we can then check for errors on without affecting the main stream
	var buf bytes.Buffer
	subStream := stream.Pool().BorrowStream(&buf)
	defer stream.Pool().ReturnStream(subStream)

	// Update the substream's indent level to match the main stream
	// we need to use reflection to do this as the indent level is not exported
	// by the jsoniter.Stream type
	//
	// (We could have simply used the subtream to check for errors, and then rewritten
	// the same object into the main stream, but this would be less efficient)
	currentPtr := unsafe.Pointer(reflect.Indirect(reflect.ValueOf(stream)).FieldByName("indention").UnsafeAddr())
	subStreamPtr := unsafe.Pointer(reflect.Indirect(reflect.ValueOf(subStream)).FieldByName("indention").UnsafeAddr())
	indent := *(*int)(currentPtr)
	*((*int)(subStreamPtr)) = indent

	// Write the value into the substream & flush it
	subStream.WriteVal(value)
	if err := subStream.Flush(); err != nil {
		return err
	}

	// Write the substream's bytes into the main stream
	stream.WriteMore()
	stream.WriteObjectField(fieldName)
	_, err := stream.Write(buf.Bytes())

	return err
}
