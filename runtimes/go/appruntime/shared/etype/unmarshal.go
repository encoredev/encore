package etype

import (
	"encoding/base64"
	stdjson "encoding/json"
	"fmt"
	"io"
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

func (u *Unmarshaller) ReadBody(body io.Reader) (payload []byte) {
	payload, err := io.ReadAll(body)
	if err == nil && len(payload) == 0 {
		u.setErr("missing request body", "request_body", fmt.Errorf("missing request body"))
	} else if err != nil {
		u.setErr("could not parse request body", "request_body", err)
	}
	return payload
}

func (u *Unmarshaller) ParseJSON(field string, iter *jsoniter.Iterator, dst any) {
	iter.ReadVal(dst)
	u.setErr("invalid json parameter", field, iter.Error)
}
