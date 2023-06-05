//go:build encore_app

package config

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

type Unmarshaler[T any] func(itr *jsoniter.Iterator, path []string) T

// CreateValue creates a new Value on the given path with the given value
func CreateValue[T any](value T, pathToValue ValuePath) Value[T] {
	valueID := Singleton.nextID()
	return func() T {
		Singleton.valueMeta(valueID, pathToValue)
		return testOverrideOrValue(valueID, value)
	}
}

// CreateValueList creates a new Value Slice on the given path with the given values
func CreateValueList[T any](value []T, pathToValue ValuePath) Values[T] {
	valueID := Singleton.nextID()
	return func() []T {
		Singleton.valueMeta(valueID, pathToValue)
		return testOverrideOrValue(valueID, value)
	}
}

// GetMetaForValue returns the ValueID and ValuePath for the given Value
func GetMetaForValue[T any](value func() T) (ValueID, ValuePath) {
	// Get the current request
	req := Singleton.rt.Current()

	// Lock so we're the only running extraction
	Singleton.extraction.mutex.Lock()
	defer Singleton.extraction.mutex.Unlock()

	// Scope the extraction to the current goroutine
	Singleton.extraction.scopeMutex.Lock()
	Singleton.extraction.forSpan = req.Req.SpanID
	Singleton.extraction.forGoRoutine = req.Goctr
	Singleton.extraction.scopeMutex.Unlock()

	// Now flag that we're running extract and need to go via the slow path in Manager.valueMeta
	Singleton.extraction.running.Store(true)
	defer func() {
		Singleton.extraction.count = 0

		// Reset the scoping
		Singleton.extraction.scopeMutex.Lock()
		Singleton.extraction.forSpan = [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
		Singleton.extraction.forGoRoutine = 0
		Singleton.extraction.scopeMutex.Unlock()

		// Release the locks
		Singleton.extraction.running.Store(false)
	}()

	// Call the value, which will trigger the extraction inside Manager.valueMeta
	value()

	// Expect exactly 1 extraction
	if Singleton.extraction.count != 1 {
		panic(fmt.Sprintf("config.Value metadata extraction failed, got %d extractions, expected 1", Singleton.extraction.count))
	}

	// Return the extracted values
	return Singleton.extraction.ExtractedID, Singleton.extraction.ExtractedPath
}

// ReadArray is a helper function that generated code can use to read an array from the JSON iterator
func ReadArray[T any](itr *jsoniter.Iterator, cb func(itr *jsoniter.Iterator, idx int) T) []T {
	rtn := make([]T, 0)

	itr.ReadArrayCB(func(iterator *jsoniter.Iterator) bool {
		rtn = append(rtn, cb(iterator, len(rtn)))
		return true
	})

	return rtn
}

// ReadMap is a helper function that generated code can use to read an map from the JSON iterator
func ReadMap[K comparable, V any](itr *jsoniter.Iterator, cb func(itr *jsoniter.Iterator, key string) (K, V)) map[K]V {
	rtn := make(map[K]V)

	itr.ReadObjectCB(func(iterator *jsoniter.Iterator, key string) bool {
		k, v := cb(iterator, key)
		rtn[k] = v
		return true
	})

	return rtn
}
