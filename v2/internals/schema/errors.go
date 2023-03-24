package schema

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"schema",
		"For more information, see https://encore.dev/docs/develop/api-schemas",

		errors.WithRangeSize(10),
	)

	errExpectedOnReciever = errRange.Newf(
		"Invalid receiver",
		"Expected exactly 1 receiver, got %d.",
	)

	errUnknownIdentifier = errRange.Newf(
		"Unknown identifier",
		"Unknown identifier `%s`",
	)

	errDeclIsntFunction = errRange.Newf(
		"Invalid declaration",
		"Declaration `%s` is not a function",
	)
)
