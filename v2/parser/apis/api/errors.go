package api

import (
	"encr.dev/pkg/errors"
)

const rawHint = `hint: signature must be func(http.ResponseWriter, *http.Request)

For more information on how to use raw APIs see https://encore.dev/docs/primitives/services-and-apis#raw-endpoints`

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

	errRawEndpointCantBePrivate = errRange.New(
		"Invalid API Directive",
		"Private APIs cannot be declared as raw endpoints.",
	)

	errWrongNumberParams = errRange.Newf(
		"Invalid API Function",
		"API functions must have at least 1 parameter, found %d parameters.",
	)

	errWrongNumberResults = errRange.Newf(
		"Invalid API Function",
		"API functions must have at most 1 or 2 results, found %d results.",
	)

	errInvalidFirstParam = errRange.New(
		"Invalid API Function",
		"The first parameter of an API function must be context.Context.",
	)

	errMultiplePayloads = errRange.New(
		"Invalid API Function",
		"API functions can only have one payload parameter.",
	)

	errInvalidPathParams = errRange.Newf(
		"Invalid API Function",
		"Expected function parameters named '%s' to match Endpoint path params.",
	)

	errLastResultMustBeError = errRange.New(
		"Invalid API Function",
		"The last result of an API function must be error.",
	)

	errInvalidRawParams = errRange.Newf(
		"Invalid API Function",
		"Raw APIs must have a two parameters of type http.ResponseWriter and *http.Request, got %d parameters.",

		errors.WithDetails(rawHint),
	)

	errInvalidRawResults = errRange.Newf(
		"Invalid API Function",
		"Raw APIs must not return any results, got %d results.",

		errors.WithDetails(rawHint),
	)

	errRawNotResponeWriter = errRange.New(
		"Invalid API Function",
		"Raw APIs must have a first parameter of type http.ResponseWriter.",

		errors.WithDetails(rawHint),
	)
	errRawNotRequest = errRange.New(
		"Invalid API Function",
		"Raw APIs must have a second parameter of type *http.Request.",

		errors.WithDetails(rawHint),
	)

	errUnexpectedParameterName = errRange.Newf(
		"Invalid API Function",
		"Unexpected parameter name %q expected %q (to match path parameter %q).",
	)

	errWildcardMustBeString = errRange.Newf(
		"Invalid API Function",
		"Wildcard parameter %q must be a string.",
	)

	errInvalidPathParamType = errRange.Newf(
		"Invalid API Function",
		"Path parameter %q must be a string, bool, integer, or encore.dev/types/uuid.UUID.",
	)
)
