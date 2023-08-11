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

// MapKeys returns the keys of the map m.
// The keys will be in an indeterminate order.
func MapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func CloseIgnore(c io.Closer) {
	_ = c.Close()
}
