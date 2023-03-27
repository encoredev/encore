package perr

import (
	"fmt"

	"encr.dev/pkg/errinsrc"
)

// ListAsErr is a wrapper around a List that implements the error interface
// allowing us to return a List as an error from functions that parse or compile
// an application.
//
// We've not implemented Error on List directly because we want to avoid accidentally
// returning a List as an error, and want to be explicit about it.
type ListAsErr struct {
	prefix string
	list   *List
}

var (
	_ error              = (*ListAsErr)(nil)
	_ errinsrc.ErrorList = (*ListAsErr)(nil) // We implement this to maintain compatibility with errinsrc detection within the Encore Platform
)

// Error returns the list of errors formatted as a single string.
func (r *ListAsErr) Error() string {
	if r.prefix != "" {
		return fmt.Sprintf("%s: %s", r.prefix, r.list.FormatErrors())
	}

	return r.list.FormatErrors()
}

// Unwrap returns the list of errors that make up this error.
//
// Note: This version of Unwrap is a Go 1.20+ feature
//
//goland:noinspection GoStandardMethods
func (r *ListAsErr) Unwrap() []error {
	rtn := make([]error, len(r.list.errs))
	for i, err := range r.list.errs {
		rtn[i] = err
	}
	return rtn
}

// As implements the As method of the error interface.
//
// It supports the following types:
//   - **ListAsErr
//   - **List
func (r *ListAsErr) As(err any) bool {
	switch err := err.(type) {
	case **ListAsErr:
		*err = r
	case **List:
		*err = r.list
	default:
		return false
	}
	return true
}

// ErrorList returns the list of errors in the source that make up this error.
func (r *ListAsErr) ErrorList() []*errinsrc.ErrInSrc {
	return r.list.errs
}
