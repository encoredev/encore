package codegen

import "encr.dev/pkg/errors"

var (
	errRange = errors.Range(
		"codegen",
		"",
	)

	errRender = errRange.New(
		"Failed to render codegen",
		"Generated code could not be parsed.",
		errors.MarkAsInternalError(),
	)
)
