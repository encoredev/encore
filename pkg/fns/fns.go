package fns

import (
	"cmp"
	"io"
	"maps"
)

// Map applies fn on all elements in src, producing a new slice
// with the results, in order.
func Map[A, B any](src []A, fn func(A) B) []B {
	dst := make([]B, len(src))
	for i, v := range src {
		dst[i] = fn(v)
	}
	return dst
}

func Max[A any, B cmp.Ordered](src []A, fn func(A) B) B {
	var m B
	for _, v := range src {
		if val := fn(v); m < val {
			m = val
		}
	}
	return m
}

// MapAndFilter applies fn on all elements in src, producing a new slice
// with the results, in order. If fn returns false, the element is
// not included in the result.
func MapAndFilter[A, B any](src []A, fn func(A) (B, bool)) []B {
	var dst []B
	for _, v := range src {
		if r, ok := fn(v); ok {
			dst = append(dst, r)
		}
	}
	return dst

}

// MapErr applies fn on all elements in src, producing a new slice
// with the results, in order. If fn returns an error, MapErr
// returns that error and the resulting slice is nil.
func MapErr[A, B any](src []A, fn func(A) (B, error)) ([]B, error) {
	dst := make([]B, len(src))
	for i, v := range src {
		var err error
		dst[i], err = fn(v)
		if err != nil {
			return nil, err
		}
	}
	return dst, nil

}

// TransformMapKeys creates a new map with the same values as m, but with
// the keys transformed by fn.
func TransformMapKeys[K1, K2 comparable, V any](m map[K1]V, fn func(K1) K2) map[K2]V {
	dst := make(map[K2]V, len(m))
	for k, v := range m {
		dst[fn(k)] = v
	}
	return dst
}

// Any returns true if any element in src satisfies the predicate.
func Any[A any](src []A, pred func(A) bool) bool {
	for _, v := range src {
		if pred(v) {
			return true
		}
	}
	return false
}

// FlatMap applies fn on all elements in src, producing a new slice
// with the results, in order.
func FlatMap[A, B any](src []A, fn func(A) []B) []B {
	var dst []B
	for _, v := range src {
		dst = append(dst, fn(v)...)
	}
	return dst
}

// Find returns the first element where pred returns true.
// The second argument is true if an element was found.
func Find[A any](src []A, pred func(A) bool) (A, bool) {
	for _, v := range src {
		if pred(v) {
			return v, true
		}
	}

	var zero A
	return zero, false
}

// Filter applies fn on all elements in src, producing a new slice
// containing the elements for which fn returned true, preserving
// the same order.
func Filter[Elem any](src []Elem, fn func(Elem) bool) []Elem {
	dst := make([]Elem, 0, len(src))
	for _, v := range src {
		if fn(v) {
			dst = append(dst, v)
		}
	}
	return dst
}

// ToMap converts a slice to a map.
func ToMap[K comparable, V any](src []V, key func(V) K) map[K]V {
	dst := make(map[K]V, len(src))
	for _, v := range src {
		dst[key(v)] = v
	}
	return dst
}

// TransformMapToSlice creates a new slice with the results of applying fn to each
// key-value pair in m.
func TransformMapToSlice[K comparable, V any, R any](m map[K]V, fn func(K, V) R) []R {
	r := make([]R, 0, len(m))
	for k, v := range m {
		r = append(r, fn(k, v))
	}
	return r
}

// MapKeys returns the keys of the map m.
// The keys will be in an indeterminate order.
func MapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

// CloseIgnore closes c, ignoring any error.
// Its main use is to satisfy linters.
func CloseIgnore(c io.Closer) {
	_ = c.Close()
}

// MergeMaps merges all maps into a single map.
func MergeMaps[K comparable, V any](ms ...map[K]V) map[K]V {
	var rtn map[K]V
	for i, m := range ms {
		if i == 0 {
			rtn = m
			continue
		}
		maps.Copy(rtn, m)
	}
	return rtn
}
