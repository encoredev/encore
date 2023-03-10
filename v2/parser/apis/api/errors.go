package api

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"api",
		`hint: valid signatures are:
	- func(context.Context) error
	- func(context.Context) (*ResponseData, error)
	- func(context.Context, *RequestData) error
	- func(context.Context, *RequestType) (*ResponseData, error)

For more information on how to use APIs, see https://encore.dev/docs/primitives/services-and-apis#defining-apis`,

		errors.WithRangeSize(50),
	)

	errDuplicateAccessOptions = errRange.Newf(
		"Invalid API Directive",
		"Multiple access options have been defined for the API; %s and %s. Pick once from %s.",
	)

	errInvalidEndpointMethod = errRange.Newf(
		"Invalid API Directive",
		"Invalid endpoint method %q.",
	)

	errEndpointMethodMustBeAllCaps = errRange.New(
		"Invalid API Directive",
		"Endpoint method must be ALLCAPS.",
	)

	errInvalidEndpointTag = errRange.New(
		"Invalid API Directive",
		"Invalid endpoint tag.",
	)

	errRawEndpointCantBePrivate = errRange.New(
		"Invalid API Directive",
		"Private APIs cannot be declared as raw endpoints.",
	)
)
