package config

import (
	"fmt"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

type Unmarshaler[T any] func(itr *jsoniter.Iterator, path []string) T

// envName takes a service name and converts it to an environment variable name in which
// the service's configuration JSON is stored at runtime
func envName(serviceName string) string {
	// normalise the name
	serviceName = strings.ToUpper(serviceName)

	return fmt.Sprintf("ENCORE_CFG_%s", serviceName)
}

func CreateValue[T any](value T, pathToValue []string) Value[T] {
	return func() T {
		return value
	}
}

func CreateValueList[T any](value []T, pathToValue []string) Values[T] {
	return func() []T {
		return value
	}
}

func ReadArray[T any](itr *jsoniter.Iterator, cb func(itr *jsoniter.Iterator, idx int) T) []T {
	rtn := make([]T, 0)

	itr.ReadArrayCB(func(iterator *jsoniter.Iterator) bool {
		rtn = append(rtn, cb(iterator, len(rtn)))
		return true
	})

	return rtn
}

func ReadMap[K comparable, V any](itr *jsoniter.Iterator, cb func(itr *jsoniter.Iterator, key string) (K, V)) map[K]V {
	rtn := make(map[K]V)

	itr.ReadObjectCB(func(iterator *jsoniter.Iterator, key string) bool {
		k, v := cb(iterator, key)
		rtn[k] = v
		return true
	})

	return rtn
}
