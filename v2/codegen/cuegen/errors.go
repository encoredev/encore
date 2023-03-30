package cuegen

import "encr.dev/pkg/errors"

var (
	errRange = errors.Range(
		"cuegen",
		"",
	)

	errNotNamedStruct = errRange.New(
		"Invalid type for config.Load",
		"The type argument passed to config.Load must be a named struct type.",
	)

	errInvalidFieldLabel = errRange.New(
		"Invalid field label",
		"The field could not be rendered as a valid CUE label.",
	)

	errInvalidCUEExpr = errRange.New(
		"Invalid CUE struct tag expression",
		"The struct tag expression is not a valid CUE expression.",
	)
)
