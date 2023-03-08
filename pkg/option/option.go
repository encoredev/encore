package option

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

type Option[T any] struct {
	Value   T
	Present bool
}

func (o *Option[T]) Clear() {
	var zero T
	o.Value = zero
	o.Present = false
}

func (o Option[T]) String() string {
	if o.Present {
		return fmt.Sprintf("%v", o.Value)
	}
	return "None"
}

func (o Option[T]) GetOrDefault(def T) T {
	if o.Present {
		return o.Value
	}
	return def
}

func (o Option[T]) MustGet() (rtn T) {
	if o.Present {
		return o.Value
	}
	panic(errors.Newf("Option value is not set: %T", rtn))
}

func (o Option[T]) IsPresent() bool {
	return o.Present
}

func (o Option[T]) Get() (T, bool) {
	return o.Value, o.Present
}

// AsOptional returns an Option where a zero value T is considered None
// and any other value is considered Some
//
// i.e.
//
//	AsOptional(nil) == None()
//	AsOptional(0) == None()
//	AsOptional(false) == None()
//	AsOptional("") == None()
//	AsOptional(&MyStruct{}) == Some(&MyStruct{})
//	AsOptional(1) == Some(1)
//	AsOptional(true) == Some(true)
func AsOptional[T comparable](v T) Option[T] {
	var zero T
	if v == zero {
		return None[T]()
	}
	return Some[T](v)
}

// Some returns an Option with the given value and present set to true
//
// This means Some(nil) is a valid present Option
// and Some(nil) != None()
func Some[T any](v T) Option[T] {
	return Option[T]{Value: v, Present: true}
}

// None returns an Option with no value set
func None[T any]() Option[T] {
	return Option[T]{Present: false}
}
