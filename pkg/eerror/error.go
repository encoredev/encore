// Package eerror stands for Encore Error and is used to provide
// a little more information about the underlying error's metadata.
//
// It also provides helper methods for working with zerolog's context
package eerror

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors/errbase"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Error struct {
	Module  string         `json:"module"`  // The module the error was raised in (normally the package name, but could be a larger "module" name)
	Message string         `json:"message"` // The message of the error, it should be human-readable and should be low entropy (i.e. so multiple errors of the same can be grouped)
	Meta    map[string]any `json:"meta"`    // Metadata about the error, this can be high entropy as it isn't used to group errors
	Stack   []*StackFrame  `json:"stack"`   // The stack trace of the error
	cause   error          `json:"-"`       // The underlying error, this is not serialized
}

var _ error = (*Error)(nil)
var _ errbase.StackTraceProvider = (*Error)(nil)

// New creates a new error with the given error
func New(module string, msg string, meta map[string]any) error {
	return &Error{
		Module:  module,
		Message: msg,
		Meta:    meta,
		Stack:   getStack(),
	}
}

// Wrap wraps the cause error with the given message and meta.
// If cause is nil, wrap returns nil
func Wrap(cause error, module string, msg string, meta map[string]any) error {
	if cause == nil {
		return nil
	}
	return &Error{
		Module:  module,
		Message: msg,
		Meta:    meta,
		Stack:   getStack(),
		cause:   cause,
	}
}

func WithMeta(err error, meta map[string]any) error {
	loopErr := err
	for loopErr != nil {
		if e, ok := loopErr.(*Error); ok {
			for key, value := range meta {
				e.Meta[key] = value
			}
			return loopErr
		}

		switch e := loopErr.(type) {
		case interface{ Unwrap() error }:
			loopErr = e.Unwrap()
		case interface{ Cause() error }:
			loopErr = e.Cause()
		default:
			loopErr = nil
		}
	}

	// if here we didn't find an *Error to add the metadata to, so we'll put on in
	return &Error{
		Module:  "",
		Message: err.Error(),
		Meta:    meta,
		Stack:   getStack(),
		cause:   err,
	}
}

// Error returns a simple string of the error
func (e *Error) Error() string {
	if e.cause != nil {
		cause := e.cause.Error()

		// Remove the module prefix if it's the same
		cause = strings.TrimPrefix(cause, "["+e.Module+"]: ")

		return fmt.Sprintf("[%s]: %s: %s", e.Module, e.Message, cause)
	}
	return fmt.Sprintf("[%s]: %s", e.Module, e.Message)
}

// Cause implements Causer for some libraries and returns the underlying cause
func (e *Error) Cause() error {
	return e.cause
}

// Unwrap implements the Go 2 unwrap interface used by xerrors and errors
func (e *Error) Unwrap() error {
	return e.cause
}

// StackTrace implements the StackTraceProvider interface for some libraries
// including ZeroLog, xerrors and Sentry
func (e *Error) StackTrace() errors.StackTrace {
	frames := make([]errors.Frame, len(e.Stack))
	for i, frame := range e.Stack {
		// Note: for historic reasons the PC is off by 1 in github.com/pkg/errors
		frames[i] = errors.Frame(frame.PC + 1)
	}
	return frames
}

// MarshalZerologObject provides a strongly-typed and encoding-agnostic interface
// to be implemented by types used with Event/Context's Object methods.
func (e *Error) MarshalZerologObject(evt *zerolog.Event) {
	LogWithMeta(evt, e)
}

// BottomStackTraceFrom returns the deepest stack trace from the given error
func BottomStackTraceFrom(err error) (rtn errors.StackTrace) {
	count := 0

	for err != nil && count < 100 {
		count++

		// If we're an error set our return data
		if e, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
			rtn = e.StackTrace()
		}

		// Recurse
		switch typed := err.(type) {
		case interface{ Unwrap() error }:
			err = typed.Unwrap()

		case interface{ Unwrap() []error }:
			errs := typed.Unwrap()
			if len(errs) > 0 {
				err = errs[0]
			} else {
				err = nil
			}

		case interface{ Cause() error }:
			err = typed.Cause()
		}
	}

	return
}

// MetaFrom will return the merged metadata from any eerror.Error objects in the errors
// given. It will unwrap errors as it descends
func MetaFrom(err error) map[string]any {
	meta := make(map[string]any)
	mergeMeta(err, meta)
	return meta
}

func mergeMeta(err error, meta map[string]any) {
	if err == nil {
		return
	}

	// Merge in the data from the deepest error first
	switch err := err.(type) {
	case interface{ Unwrap() error }:
		mergeMeta(err.Unwrap(), meta)

	case interface{ Cause() error }:
		mergeMeta(err.Cause(), meta)
	}

	// Then merge in our data
	if e, ok := err.(*Error); ok {
		for key, value := range e.Meta {
			meta[key] = value
		}
	}

}
