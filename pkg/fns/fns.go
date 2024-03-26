package fns

import "io"

// Map applies fn on all elements in src, producing a new slice
// with the results, in order.
func Map[A, B any](src []A, fn func(A) B) []B {
	dst := make([]B, len(src))
	for i, v := range src {
		dst[i] = fn(v)
	}
	return dst
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
