//go:build !encore_internal

package encore

// CurrentRequest returns the Request that is currently being handled by the calling Go routine
//
// It is safe for concurrent use and will return a new Request on each evocation, so can be mutated by the
// calling code without impacting future calls.
//
// CurrentRequest never returns nil.
func CurrentOp() *Request {
	panic("encore apps must be run using the encore command")
}
