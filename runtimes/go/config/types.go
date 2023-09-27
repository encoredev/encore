package config

import (
	"time"

	"encore.dev/types/uuid"
)

// Value represents a value in the configuration for this application
// which can be any value represented in the configuration files.
//
// It is a function because the underlying value could change while
// the application is still running due to unit tests providing
// overrides to test different behaviours. To change the value within
// a single unit test, use the et.SetCfg function.
type Value[T any] func() T

// Values represents a list of values in the configuration for this
// application which can be any value represented in the configuration files.
//
// It is a function because the underlying value could change while
// the application is still running due to unit tests providing
// overrides to test different behaviours.
type Values[T any] func() []T

/*
The following types represent syntax sugar and are all just shorthand for
Value[T]
*/

type Bool = Value[bool]
type Int8 = Value[int8]
type Int16 = Value[int16]
type Int32 = Value[int32]
type Int64 = Value[int64]
type Uint8 = Value[uint8]
type Uint16 = Value[uint16]
type Uint32 = Value[uint32]
type Uint64 = Value[uint64]
type Float32 = Value[float32]
type Float64 = Value[float64]
type String = Value[string]
type Bytes = Value[[]byte]
type Time = Value[time.Time]
type UUID = Value[uuid.UUID]
type Int = Value[int]
type Uint = Value[uint]
