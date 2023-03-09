package authhandler

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"authhandler",
		`hint: valid signatures are:
	- func(ctx context.Context, p *Params) (auth.UID, error)
	- func(ctx context.Context, p *Params) (auth.UID, *UserData, error)
	- func(ctx context.Context, token string) (auth.UID, error)
	- func(ctx context.Context, token string) (auth.UID, *UserData, error)

note: *Params and *UserData are custom data types you define

For more information checkout https://encore.dev/docs/develop/auth`,

		errors.WithRangeSize(20),
	)

	errInvalidNumberParameters = errRange.Newf(
		"Invalid auth handler Signature",
		"The auth handler must have exactly 2 parameters, found %d.",
	)

	errInvalidNumberResults = errRange.Newf(
		"Invalid auth handler Signature",
		"The auth handler must have 2 or 3 result parameters, found %d.",
	)

	errInvalidFirstParameter = errRange.New(
		"Invalid auth handler Signature",
		"The first parameter must be a context.Context.",
	)

	errInvalidSecondParameter = errRange.New(
		"Invalid auth handler Signature",
		"The second parameter must be a string or a pointer to a named struct.",
	)

	errInvalidFirstResult = errRange.New(
		"Invalid auth handler Signature",
		"The first result must be of type auth.UID.",
	)

	errInvalidSecondResult = errRange.New(
		"Invalid auth handler Signature",
		"The second result must be a pointer to a named struct.",
	)

	errInvalidFieldTags = errRange.New(
		"Invalid auth payload",
		"All fields used within an auth payload must originate from either an HTTP header or a query parameter.",
		errors.WithDetails("You can specify them for each field using the struct tags, for example with `header:\"X-My-Header\"` or `query:\"my-query\"`.\n\nFor more information checkout https://encore.dev/docs/develop/auth"),
	)
)
