package errs

// ErrDetails is a marker interface for telling Encore
// the type is used for reporting error details.
//
// We require a marker method (as opposed to using interface{})
// to facilitate static analysis and to ensure the type
// can be properly serialized across the network.
type ErrDetails interface {
	ErrDetails() // marker method; it need not do anything
}
