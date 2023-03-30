package selector

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"selector",
		"",

		errors.WithRangeSize(10),
	)

	errMissingSelectorType = errRange.New(
		"Invalid Selector",
		"Missing selector type.",
	)

	errUnknownSelectorType = errRange.Newf(
		"Invalid Selector",
		"Unknown selector type %q.",
	)

	errInvalidSelectorValue = errRange.Newf(
		"Invalid Selector",
		"Invalid selector value %q.",
	)
)
