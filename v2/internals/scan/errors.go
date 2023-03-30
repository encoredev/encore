package scan

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"scan",
		"",
		errors.WithRangeSize(25),
	)

	errResolvingModulePath = errRange.New(
		"Error Resolving Module Path",
		"An error occurred while trying to resolve the module path.",
	)
)
