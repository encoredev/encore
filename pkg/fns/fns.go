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
