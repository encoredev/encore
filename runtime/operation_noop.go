//go:build !encore_internal

package encore

// CurrentOp returns the Operation which was the root cause of why the current code is running.
//
// It is thread safe and will return a new Operation on each evocation, so can be mutated by the
// calling code without impacting future calls.
//
// CurrentOp will never return nil.
func CurrentOp() *Operation {
	panic("encore apps must be run using the encore command")
}
