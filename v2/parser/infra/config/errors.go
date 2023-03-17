package config

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"config",
		"For more information on configuration, see https://encore.dev/docs/develop/config",

		errors.WithRangeSize(25),
	)

	errInvalidLoad = errRange.New(
		"Invalid call to config.Load[T]()",
		"A call to config.Load[T]() does not accept any arguments.",
	)

	errInvalidConfigType = errRange.New(
		"Invalid call to config.Load[T]()",
		"A call to config.Load[T]() must be passed a named struct type as it's argument.",
	)

	ErrConfigUsedOutsideOfService = errRange.New(
		"Invalid call to config.Load[T]()",
		"A call to config.Load[T]() can only be made from within a service.",
	)

	ErrConfigUsedInSubPackage = errRange.New(
		"Invalid call to config.Load[T]()",
		"A call to config.Load[T]() can only be made from the top level package of a service.",
	)

	ErrCrossServiceConfigUse = errRange.New(
		"Cross service config use",
		"A config instance can only be referenced from within the service that the call to `config.Load[T]()` was made in.",
	)

	errUnexportedField = errRange.New(
		"Invalid config type",
		"Field is not exported and is in a datatype which is used by a call to `config.Load[T]()`. Unexported fields cannot be initialised by Encore, thus are not allowed in this context.",
	)

	errAnonymousField = errRange.New(
		"Invalid config type",
		"Field is an anonymous field and is in a datatype which is used by a call to `config.Load[T]()`. Anonymous fields cannot be initialised by Encore, thus are not allowed in this context.",
	)

	errInvalidConfigTypeUsed = errRange.New(
		"Invalid config type",
		"Types used within data structures which are used by a call to `config.Load[T]()` must either be a built-in type a inline struct or a named struct type.",
	)

	errNestedValueUsage = errRange.New(
		"Invalid config type",
		"The type of config.Value[T] cannot be another config.Value[T]",
	)
)
