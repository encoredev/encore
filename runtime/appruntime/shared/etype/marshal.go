package etype

import (
	"encoding/base64"
	stdjson "encoding/json"
	"strconv"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/types/uuid"
)

type ElemMarshaller[T any] func(T) string

func MarshalOne[T any](fn ElemMarshaller[T], data T) string {
	return fn(data)
}

func MarshalList[T any](fn ElemMarshaller[T], data []T) []string {
	result := make([]string, len(data))
	for i, x := range data {
		result[i] = fn(x)
	}
	return result
}

func MarshalInt16(s int16) (v string) {
	return strconv.FormatInt(int64(s), 10)
}

func MarshalUint16(s uint16) (v string) {
	return strconv.FormatUint(uint64(s), 10)
}

func MarshalFloat64(s float64) (v string) {
	return strconv.FormatFloat(s, uint8(0x66), -1, 64)
}

func MarshalBytes(s []byte) (v string) {
	return base64.URLEncoding.EncodeToString(s)
}

func MarshalUserID(s auth.UID) (v string) {
	return string(s)
}

func MarshalUint32(s uint32) (v string) {
	return strconv.FormatUint(uint64(s), 10)
}

func MarshalString(s string) (v string) {
	return s
}

func MarshalUUID(s uuid.UUID) (v string) {
	return s.String()
}

func MarshalJSON(s stdjson.RawMessage) (v string) {
	return string(s)
}

func MarshalInt(s int) (v string) {
	return strconv.FormatInt(int64(s), 10)
}

func MarshalUint(s uint) (v string) {
	return strconv.FormatUint(uint64(s), 10)
}

func MarshalBool(s bool) (v string) {
	return strconv.FormatBool(s)
}

func MarshalUint8(s uint8) (v string) {
	return strconv.FormatUint(uint64(s), 10)
}

func MarshalUint64(s uint64) (v string) {
	return strconv.FormatUint(s, 10)
}

func MarshalFloat32(s float32) (v string) {
	return strconv.FormatFloat(float64(s), uint8(0x66), -1, 32)
}

func MarshalTime(s time.Time) (v string) {
	return s.Format(time.RFC3339)
}

func MarshalInt8(s int8) (v string) {
	return strconv.FormatInt(int64(s), 10)
}

func MarshalInt32(s int32) (v string) {
	return strconv.FormatInt(int64(s), 10)
}

func MarshalInt64(s int64) (v string) {
	return strconv.FormatInt(s, 10)
}
