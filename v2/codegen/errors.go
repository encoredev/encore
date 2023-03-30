package codegen

import "encr.dev/pkg/errors"

var (
	errRange = errors.Range(
		"codegen",
		"",
	)

	errRender = errRange.Newf(
		"Failed to render codegen",
		"Generated code could not be parsed.",
		errors.MarkAsInternalError(),
	)
)
