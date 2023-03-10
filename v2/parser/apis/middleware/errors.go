package middleware

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"middleware",
		"hint: middleware must have the signature:\n\t"+
			"func(req middleware.Request, next middleware.Next) middleware.Response\n\n"+
			"For more information on how to use middleware, see https://encore.dev/docs/develop/middleware",

		errors.WithRangeSize(20),
	)

	errInvalidSelectorType = errRange.Newf(
		"Invalid middleware selector",
		"Middleware target only supports tags a selectors (got '%s').",
	)

	errWrongNumberParams = errRange.Newf(
		"Invalid middleware function",
		"Middleware functions must have exactly 2 parameters, found %d parameters.",
	)

	errWrongNumberResults = errRange.Newf(
		"Invalid middleware function",
		"Middleware functions must have exactly 1 result, found %d results.",
	)

	errInvalidFirstParam = errRange.New(
		"Invalid middleware function",
		"The first parameter of a middleware function must be middleware.Request.",
	)

	errInvalidSecondParam = errRange.New(
		"Invalid middleware function",
		"The second parameter of a middleware function must be middleware.Next.",
	)

	errInvalidReturnType = errRange.New(
		"Invalid middleware function",
		"The return type of a middleware function must be middleware.Response.",
	)
)
