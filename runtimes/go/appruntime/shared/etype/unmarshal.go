package etype

import (
	"encoding"
	"encoding/base64"
	stdjson "encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/beta/auth"
	"encore.dev/types/option"
	"encore.dev/types/uuid"
)

type ElemUnmarshaller[T any] func(s string) (T, error)

func UnmarshalOne[T any](u *Unmarshaller, fn ElemUnmarshaller[T], field, data string, required bool) (val T) {
	if !required && data == "" {
		return val
	}

	u.NonEmptyValues++
	val, err := fn(data)
	if err != nil {
		u.setErr("invalid parameter", field, err)
	}
	return val
}

func UnmarshalList[T any](u *Unmarshaller, fn ElemUnmarshaller[T], field string, data []string, required bool) []T {
	if !required && len(data) == 0 {
		return nil
	}
	u.NonEmptyValues++
	var list []T
	for _, x := range data {
		val, err := fn(x)
		if err != nil {
			u.setErr("invalid parameter", field, err)
			break
		}
		list = append(list, val)
	}
	return list
}

// OptionUnmarshaller wraps an ElemUnmarshaller to produce an ElemUnmarshaller for option.Option types.
func OptionUnmarshaller[T any](inner ElemUnmarshaller[T]) ElemUnmarshaller[option.Option[T]] {
	return func(s string) (option.Option[T], error) {
		v, err := inner(s)
		if err != nil {
			return option.None[T](), err
		}
		return option.Some(v), nil
	}
}

// IncNonEmpty increments the number of non-empty values this decoder has decoded.
// It's useful when decoding something that doesn't go through the unmarshaller
// but still was present, so that code that checks NonEmptyValues is correct.
func (u *Unmarshaller) IncNonEmpty() {
	u.NonEmptyValues++
}

// Unmarshaller is used to serialize request data into strings and deserialize response data from strings
type Unmarshaller struct {
	Error          error // The last error that occurred
	NonEmptyValues int   // The number of values this decoder has decoded
}

func UnmarshalInt16(s string) (int16, error) {
	x, err := strconv.ParseInt(s, 10, 16)
	return int16(x), err
}

func UnmarshalUint16(s string) (uint16, error) {
	x, err := strconv.ParseUint(s, 10, 16)
	return uint16(x), err
}

func UnmarshalFloat64(s string) (v float64, err error) {
	x, err := strconv.ParseFloat(s, 64)
	return x, err
}

func UnmarshalBytes(s string) ([]byte, error) {
	v, err := base64.URLEncoding.DecodeString(s)
	return v, err
}

func UnmarshalUserID(s string) (auth.UID, error) {
	return auth.UID(s), nil
}

func UnmarshalUint32(s string) (uint32, error) {
	x, err := strconv.ParseUint(s, 10, 32)
	return uint32(x), err
}

func UnmarshalString(s string) (string, error) {
	return s, nil
}

func UnmarshalUUID(s string) (uuid.UUID, error) {
	v, err := uuid.FromString(s)
	return v, err
}

func UnmarshalJSON(s string) (stdjson.RawMessage, error) {
	return stdjson.RawMessage(s), nil
}

func UnmarshalInt(s string) (int, error) {
	x, err := strconv.ParseInt(s, 10, 64)
	return int(x), err
}

func UnmarshalUint(s string) (uint, error) {
	x, err := strconv.ParseUint(s, 10, 64)
	return uint(x), err
}

func UnmarshalBool(s string) (bool, error) {
	v, err := strconv.ParseBool(s)
	return v, err
}

func UnmarshalUint8(s string) (uint8, error) {
	x, err := strconv.ParseUint(s, 10, 8)
	return uint8(x), err
}

func UnmarshalUint64(s string) (uint64, error) {
	x, err := strconv.ParseUint(s, 10, 64)
	return x, err
}

func UnmarshalFloat32(s string) (float32, error) {
	x, err := strconv.ParseFloat(s, 32)
	return float32(x), err
}

func UnmarshalTime(s string) (time.Time, error) {
	v, err := time.Parse(time.RFC3339, s)
	return v, err
}

func UnmarshalInt8(s string) (int8, error) {
	x, err := strconv.ParseInt(s, 10, 8)
	return int8(x), err
}

func UnmarshalInt32(s string) (int32, error) {
	x, err := strconv.ParseInt(s, 10, 32)
	return int32(x), err
}

func UnmarshalInt64(s string) (int64, error) {
	x, err := strconv.ParseInt(s, 10, 64)
	return x, err
}

// setErr sets the error within the object if one is not already set
func (u *Unmarshaller) setErr(msg, field string, err error) {
	if err != nil && u.Error == nil {
		u.Error = fmt.Errorf("%s: %s: %w", field, msg, err)
	}
}

// BodyError marks a failure to read or parse the request body.
// Its text can name Go struct fields, JSON parser internals and an excerpt
// of the payload, so callers must keep it out of responses to external clients.
type BodyError struct {
	Err error
	// Detail is a description that is safe to show to clients,
	// expressed only in terms of the API contract. It may be empty.
	Detail string
}

func (e *BodyError) Error() string { return e.Err.Error() }
func (e *BodyError) Unwrap() error { return e.Err }

// PublicDetail returns the client-safe description of the error. It may be empty.
// It lets packages that cannot import etype detect a BodyError.
func (e *BodyError) PublicDetail() string { return e.Detail }

// setBodyErr is like setErr but marks the error as a [BodyError].
func (u *Unmarshaller) setBodyErr(msg, field string, err error, detail string) {
	if err != nil && u.Error == nil {
		u.Error = &BodyError{Err: fmt.Errorf("%s: %s: %w", field, msg, err), Detail: detail}
	}
}

func (u *Unmarshaller) ReadBody(body io.Reader) (payload []byte) {
	payload, err := io.ReadAll(body)
	if err == nil && len(payload) == 0 {
		u.setBodyErr("missing request body", "request_body", fmt.Errorf("missing request body"), "missing request body")
	} else if err != nil {
		u.setBodyErr("could not parse request body", "request_body", err, "")
	}
	return payload
}

// ParseJSON reads the next JSON value into dst. field is the name of the field in the API.
func (u *Unmarshaller) ParseJSON(field string, iter *jsoniter.Iterator, dst any) {
	iter.ReadVal(dst)
	if iter.Error != nil {
		u.setBodyErr("invalid json parameter", field, iter.Error, jsonFieldDetail(field, dst))
	}
}

// jsonFieldDetail describes a field that failed to decode, without naming Go identifiers.
// jsoniter reports syntax and type errors alike as text, so it only says what the field expects.
func jsonFieldDetail(field string, dst any) string {
	if want := jsonTypeName(dst); want != "" {
		return fmt.Sprintf("invalid value for field %q: expected %s", field, want)
	}
	return fmt.Sprintf("invalid value for field %q", field)
}

var (
	jsonUnmarshalerType = reflect.TypeFor[stdjson.Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

// jsonTypeName names the JSON type that dst (a pointer) expects, or "" if that is not
// predictable, as for types with their own unmarshalling.
func jsonTypeName(dst any) string {
	t := reflect.TypeOf(dst).Elem()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if p := reflect.PointerTo(t); p.Implements(jsonUnmarshalerType) || p.Implements(textUnmarshalerType) {
		return ""
	}
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "a boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "an integer"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return "a base64-encoded string"
		}
		return "an array"
	case reflect.Array:
		return "an array"
	case reflect.Map, reflect.Struct:
		return "an object"
	}
	return ""
}
