// Package option provides a generic Option type for representing optional values
// in a more type-safe way than using pointers or zero values.
package option

import (
	"encoding/json"
	"fmt"
)

// Option is a type that represents a value that may or may not be present.
type Option[T any] struct {
	value   T
	present bool
}

func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.present {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

func (o *Option[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		var zero T
		o.present = false
		o.value = zero
		return nil
	}

	o.present = true
	return json.Unmarshal(data, &o.value)
}

// FromComparable returns Some(v) if v is not zero, and None otherwise.
// If T implements an IsZero() bool method, that is also used to determine if v is zero.
func FromComparable[T comparable](v T) Option[T] {
	var zero T
	if v == zero {
		return None[T]()
	} else if z, ok := any(v).(interface{ IsZero() bool }); ok && z.IsZero() {
		return None[T]()
	}
	return Some[T](v)
}

// FromPointer returns Some(*v) if v is not nil, and None otherwise.
func FromPointer[T any](v *T) Option[T] {
	if v == nil {
		return None[T]()
	}
	return Some[T](*v)
}

// Some returns an Option with the given value and present set to true.
func Some[T any](v T) Option[T] {
	return Option[T]{value: v, present: true}
}

// None returns an Option with no value set.
func None[T any]() Option[T] {
	return Option[T]{present: false}
}

// IsSome returns true if the Option has a value set.
func (o Option[T]) IsSome() bool {
	return o.present
}

// IsNone returns true if the Option has no value set.
func (o Option[T]) IsNone() bool {
	return !o.present
}

// IsZero is an alias for IsNone, to support usage in structs with "omitempty".
func (o Option[T]) IsZero() bool {
	return !o.present
}

// Get gets the option value and returns ok==true if present.
// Commonly used in the "comma ok" idiom:
//
//	if val, ok := option.Get(); ok {
//	    ...
//	}
func (o Option[T]) Get() (val T, ok bool) {
	return o.value, o.present
}

// GetOrElse returns the value if present, otherwise returns alternative.
func (o Option[T]) GetOrElse(alternative T) T {
	if o.present {
		return o.value
	}
	return alternative
}

// GetOrElseF returns the value if present, otherwise returns alternative().
func (o Option[T]) GetOrElseF(alternative func() T) T {
	if o.present {
		return o.value
	}
	return alternative()
}

// MustGet returns the value if present, and otherwise panics.
func (o Option[T]) MustGet() T {
	if o.present {
		return o.value
	}
	panic("option is None")
}

// OrElse returns an Option with the value if present, otherwise returns Some(alternative).
func (o Option[T]) OrElse(alternative T) Option[T] {
	if o.present {
		return o
	}
	return Some(alternative)
}

// Contains returns pred(v) if the option contains v, and false otherwise.
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
	return "null"
}

func (o Option[T]) GoString() string {
	if o.present {
		return fmt.Sprintf("option.Some(%v)", o.value)
	}
	return fmt.Sprintf("option.None[%T]()", o.value)
}

// PtrOrNil returns the value as a pointer if present, or nil otherwise.
func (o Option[T]) PtrOrNil() *T {
	if o.present {
		return &o.value
	}
	return nil
}

// Equal reports whether a and b are equal, using ==.
// If both are None, they are considered equal.
func Equal[T comparable](a, b Option[T]) bool {
	if a.present != b.present {
		return false
	}
	if !a.present {
		return true
	}
	return a.value == b.value
}

// Contains returns true if the option is present and matches the given value.
func Contains[T comparable](option Option[T], matches T) bool {
	if option.present {
		return option.value == matches
	}
	return false
}

// Map returns an Option with the value mapped by the given function if present, otherwise returns None.
func Map[T, R any](option Option[T], f func(T) R) Option[R] {
	if option.present {
		return Some(f(option.value))
	}
	return None[R]()
}
