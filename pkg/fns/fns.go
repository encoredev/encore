package fns

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
