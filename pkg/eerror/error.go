// Package eerror stands for Encore Error and is used to provide
// a little more information about the underlying error's metadata.
//
// It also provides helper methods for working with zerolog's context
package eerror

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

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
	return fmt.Sprintf("%s: %s", e.Module, e.Message)
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
		frames[i] = errors.Frame(frame.PC)
	}
	return frames
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

// LogWithMeta merges in the metadata from the errors into the log context
func LogWithMeta(evt *zerolog.Event, err error) *zerolog.Event {
	if err == nil {
		return evt
	}

	evt = evt.Err(err)
	meta := MetaFrom(err)
	for key, value := range meta {
		switch value := value.(type) {
		case json.RawMessage:
			evt = evt.RawJSON(key, value)
		case error:
			evt = evt.AnErr(key, value)
		case time.Time:
			evt = evt.Time(key, value)
		case time.Duration:
			evt = evt.Dur(key, value)
		case net.IP:
			evt = evt.IPAddr(key, value)
		case net.IPNet:
			evt = evt.IPPrefix(key, value)
		case net.HardwareAddr:
			evt = evt.MACAddr(key, value)
		case string:
			evt = evt.Str(key, value)
		case int:
			evt = evt.Int(key, value)
		case int8:
			evt = evt.Int8(key, value)
		case int16:
			evt = evt.Int16(key, value)
		case int32:
			evt = evt.Int32(key, value)
		case int64:
			evt = evt.Int64(key, value)
		case uint:
			evt = evt.Uint(key, value)
		case uint8:
			evt = evt.Uint8(key, value)
		case uint16:
			evt = evt.Uint16(key, value)
		case uint32:
			evt = evt.Uint32(key, value)
		case uint64:
			evt = evt.Uint64(key, value)
		case float32:
			evt = evt.Float32(key, value)
		case float64:
			evt = evt.Float64(key, value)
		case bool:
			evt = evt.Bool(key, value)
		case []error:
			evt = evt.Errs(key, value)
		case []time.Time:
			evt = evt.Times(key, value)
		case []time.Duration:
			evt = evt.Durs(key, value)
		case []string:
			evt = evt.Strs(key, value)
		case []int:
			evt = evt.Ints(key, value)
		case []int8:
			evt = evt.Ints8(key, value)
		case []int16:
			evt = evt.Ints16(key, value)
		case []int32:
			evt = evt.Ints32(key, value)
		case []int64:
			evt = evt.Ints64(key, value)
		case []uint:
			evt = evt.Uints(key, value)
		case []byte: // uint8 / byte are the same thing so we'll default to bytes
			evt = evt.Bytes(key, value)
		case []uint16:
			evt = evt.Uints16(key, value)
		case []uint32:
			evt = evt.Uints32(key, value)
		case []uint64:
			evt = evt.Uints64(key, value)
		case []float32:
			evt = evt.Floats32(key, value)
		case []float64:
			evt = evt.Floats64(key, value)
		case []bool:
			evt = evt.Bools(key, value)
		default:
			evt = evt.Interface(key, value)
		}
	}
	return evt
}
