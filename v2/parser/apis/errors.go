package apis

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"parser/apis",
		"",
	)

	errInvalidDirective = errRange.New(
		"Invalid directive",
		"",
	)

	errUnexpectedDirective = errRange.Newf(
		"Invalid directive",
		"Unexpected directive %q on function declaration.",
	)
)
