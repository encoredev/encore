package tracedfunc

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"tracing",
		"",
		errors.WithRangeSize(20),
	)

	errNonGoFunc = errRange.New(
		"Invalid traced function",
		"Traced functions must be Go functions (not C or assembly) or interface functions.",
	)

	errInvalidTracedFunc = errRange.New(
		"Invalid trace type",
		"Traced functions must be one of the following types: internal, request_handler, call, producer or consumer.",
	)
)
