package config

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"config",
		"For more information on configuration, see https://encore.dev/docs/develop/config",

		errors.WithRangeSize(10),
	)

	errInvalidLoad = errRange.New(
		"Invalid call to config.Load[T]()",
		"A call to config.Load[T]() does not accept any arguments.",
	)

	errInvalidConfigType = errRange.New(
		"Invalid call to config.Load[T]()",
		"A call to config.Load[T]() must be passed a named struct type as it's argument.",
	)
)
