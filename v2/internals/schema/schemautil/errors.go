package schemautil

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"schema",
		"For more information, see https://encore.dev/docs/develop/api-schemas",

		errors.WithRangeSize(10),
	)

	errMissingTypeArg = errRange.New(
		"Missing type argument",
		"Missing type argument",
	)
)
