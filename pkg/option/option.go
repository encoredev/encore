package option

import (
	"fmt"
	"reflect"

	"github.com/cockroachdb/errors"
)

// Option is a type that represents a value that may or may not be present
//
// It is different a normal value as it can distinguish between a zero value and a missing value
// even on pointer types
type Option[T any] struct {
	value   T
	present bool
}

// Equal reports whether two fields are equal.
// It's implemented for testing purposes.
func (o Option[T]) Equal(other Option[T]) bool {
	if !o.present && !other.present {
		return true
	} else if o.present != other.present {
		return false
	}

	val := reflect.ValueOf(o.value)

	// If there is an Equal method, call it.
	// We have to use reflection because Go doesn't support type assertions
	// like o.value.(equaler[T]).
	if equal := val.MethodByName("Equal"); equal.IsValid() {
		// Make sure it looks like the right shape.
		if equal.Kind() == reflect.Func && equal.Type().NumIn() == 1 {
			result := equal.Call([]reflect.Value{reflect.ValueOf(other.value)})
			return result[0].Bool()
		}
	}

	// Make sure the types are comparable since we don't enforce
	// that on the type constraint.
	if val.Comparable() {
		return val.Equal(reflect.ValueOf(other.value))
	}
	return false
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

// FromPointer returns an Option where a nil pointer is considered None
// and any other value is considered Some, with the value dereferenced.
func FromPointer[T any](v *T) Option[T] {
	if v == nil {
		return None[T]()
	}
	return Some[T](*v)
}

// Some returns an Option with the given value and present set to true
//
// This means Some(nil) is a valid present Option
// and Some(nil) != None()
func Some[T any](v T) Option[T] {
	return Option[T]{value: v, present: true}
}

// None returns an Option with no value set
func None[T any]() Option[T] {
	return Option[T]{present: false}
}

// CommaOk is a helper function to convert a comma ok idiom into an Option.
// If ok is true it returns Some(v), otherwise it returns None.
func CommaOk[T any](v T, ok bool) Option[T] {
	if ok {
		return Some[T](v)
	}
	return None[T]()
}

// Present returns true if the Option has a value set
func (o Option[T]) Present() bool {
	return o.present
}

// Empty returns true if the Option has no value set
func (o Option[T]) Empty() bool {
	return !o.present
}

// OrElse returns an Option with the value if present, otherwise returns the alternative value
func (o Option[T]) OrElse(alternative T) Option[T] {
	if o.present {
		return o
	}
	return Some(alternative)
}

// Get gets the option value and returns ok==true if present.
func (o Option[T]) Get() (val T, ok bool) {
	return o.value, o.present
}

// GetOrElse returns the value if present, otherwise returns the alternative value
func (o Option[T]) GetOrElse(alternative T) T {
	if o.present {
		return o.value
	}
	return alternative
}

// GetOrElseF returns the value if present, otherwise returns the alternative value
func (o Option[T]) GetOrElseF(alternative func() T) T {
	if o.present {
		return o.value
	}
	return alternative()
}

// MustGet returns the value if present, otherwise panics
func (o Option[T]) MustGet() (rtn T) {
	if o.present {
		return o.value
	}
	panic(errors.Newf("Option value is not set: %T", rtn))
}

// ForAll calls the given function with the value if present
func (o Option[T]) ForAll(f func(v T)) {
	if o.present {
		f(o.value)
	}
}

// ForEach returns true if the Option is empty or the given predicate returns true on the value
func (o Option[T]) ForEach(predicate func(v T) bool) bool {
	if o.present {
		return predicate(o.value)
	}
	return true
}

// Contains returns true if the Option is present and the given predicate returns true on the value
// otherwise returns false
func (o Option[T]) Contains(predicate func(v T) bool) bool {
	if o.present {
		return predicate(o.value)
	}
	return false
}

func (o Option[T]) String() string {
	if o.present {
		return fmt.Sprintf("%v", o.value)
	}
	return "None"
}

// PtrOrNil returns the value as a pointer, if present, or nil otherwise.
func (o Option[T]) PtrOrNil() *T {
	if o.present {
		return &o.value
	}
	return nil
}
