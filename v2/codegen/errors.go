package codegen

import "encr.dev/pkg/errors"

var (
	errRange = errors.Range(
		"codegen",
		"",
	)

	errRender = errRange.Newf(
		"Failed to render codegen",
		"Expected exactly 1 receiver, got %d.",
		errors.MarkAsInternalError(),
	)
)
