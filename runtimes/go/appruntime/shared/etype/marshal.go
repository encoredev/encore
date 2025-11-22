package etype

import (
	"encoding/base64"
	stdjson "encoding/json"
	"strconv"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/types/option"
	"encore.dev/types/uuid"
)

type ElemMarshaller[T any] func(T) (val string, present bool)

func MarshalOne[T any](fn ElemMarshaller[T], data T) string {
	if val, present := fn(data); present {
		return val
	} else {
		return ""
	}
}

func MarshalOneAsList[T any](fn ElemMarshaller[T], data T) []string {
	if val, present := fn(data); present {
		return []string{val}
	} else {
		return []string{}
	}
}

func MarshalList[T any](fn ElemMarshaller[T], data []T) []string {
	result := make([]string, 0, len(data))
	for _, x := range data {
		if val, present := fn(x); present {
			result = append(result, val)
		}
	}
	return result
}

// OptionMarshaller wraps an ElemMarshaller to produce an ElemMarshaller for option.Option types.
func OptionMarshaller[T any](inner ElemMarshaller[T]) ElemMarshaller[option.Option[T]] {
	return func(s option.Option[T]) (val string, present bool) {
		if val, ok := s.Get(); ok {
			return inner(val)
		}
		return "", false
	}
}

func MarshalInt16(s int16) (v string, present bool) {
	return strconv.FormatInt(int64(s), 10), true
}

func MarshalUint16(s uint16) (v string, present bool) {
	return strconv.FormatUint(uint64(s), 10), true
}

func MarshalFloat64(s float64) (v string, present bool) {
	return strconv.FormatFloat(s, uint8(0x66), -1, 64), true
}

func MarshalBytes(s []byte) (v string, present bool) {
	return base64.URLEncoding.EncodeToString(s), true
}

func MarshalUserID(s auth.UID) (v string, present bool) {
	return string(s), true
}

func MarshalUint32(s uint32) (v string, present bool) {
	return strconv.FormatUint(uint64(s), 10), true
}

func MarshalString(s string) (v string, present bool) {
	return s, true
}

func MarshalUUID(s uuid.UUID) (v string, present bool) {
	return s.String(), true
}

func MarshalJSON(s stdjson.RawMessage) (v string, present bool) {
	return string(s), true
}

func MarshalInt(s int) (v string, present bool) {
	return strconv.FormatInt(int64(s), 10), true
}

func MarshalUint(s uint) (v string, present bool) {
	return strconv.FormatUint(uint64(s), 10), true
}

func MarshalBool(s bool) (v string, present bool) {
	return strconv.FormatBool(s), true
}

func MarshalUint8(s uint8) (v string, present bool) {
	return strconv.FormatUint(uint64(s), 10), true
}

func MarshalUint64(s uint64) (v string, present bool) {
	return strconv.FormatUint(s, 10), true
}

func MarshalFloat32(s float32) (v string, present bool) {
	return strconv.FormatFloat(float64(s), uint8(0x66), -1, 32), true
}

func MarshalTime(s time.Time) (v string, present bool) {
	return s.Format(time.RFC3339), true
}

func MarshalInt8(s int8) (v string, present bool) {
	return strconv.FormatInt(int64(s), 10), true
}

func MarshalInt32(s int32) (v string, present bool) {
	return strconv.FormatInt(int64(s), 10), true
}

func MarshalInt64(s int64) (v string, present bool) {
	return strconv.FormatInt(s, 10), true
}
