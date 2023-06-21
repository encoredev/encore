package apienc

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"apienc",
		"For more information on API schemas, see https://encore.dev/docs/develop/api-schemas",
		errors.WithRangeSize(20),
	)

	errAnonymousFields = errRange.New(
		"Invalid API schema",
		"Anonymous fields in top-level request/response types are not supported.",
	)

	errTagConflict = errRange.Newf(
		"Invalid API schema",
		"The tag \"%s\" cannot be used with the tag \"%s\".",
	)

	errResponseMustBeNamedStruct = errRange.New(
		"Invalid response type",
		"API response types must be named structs.",
	)

	errResponseTypeMustOnlyBeBodyOrHeaders = errRange.New(
		"Invalid response type",
		"API response type must only contain a body or headers parameters.",
	)

	errRequestMustBeNamedStruct = errRange.New(
		"Invalid request type",
		"API request types must be named structs.",
	)

	errRequestInvalidLocation = errRange.New(
		"Invalid request type",
		"API request must only contain query, body, and header parameters.",
	)

	errReservedHeaderPrefix = errRange.New(
		"Use of reserved header prefix",
		"HTTP headers starting with \"X-Encore\" are reserved for internal use.",
	)

	ErrFuncNotSupported = errRange.New(
		"Invalid API schema",
		"Functions are not supported in API schemas.",
	)

	ErrInterfaceNotSupported = errRange.New(
		"Invalid API schema",
		"Interfaces are not supported in API schemas.",
	)

	ErrAnonymousFieldsNotSupported = errRange.New(
		"Invalid API schema",
		"Anonymous fields are not supported in API schemas.",
	)

	errInvalidHeaderType = errRange.Newf(
		"Invalid request type",
		"API request parameters of type %s are not supported in headers. You can only "+
			"use built-in types types such as strings, booleans, int, time.Time.",

		errors.WithDetails("See https://encore.dev/docs/develop/api-schemas#supported-types for more information."),
	)

	errInvalidQueryStringType = errRange.Newf(
		"Invalid request type",
		"API request parameters of type %s are not supported in query strings. You can only "+
			"use built-in types, or slices of built-in types such as strings, booleans, int, time.Time.",

		errors.WithDetails("APIs which are sent as GET, HEAD or DELETE requests are unable to contain JSON bodies, "+
			"thus all parameters must be sent as query strings or headers. "+
			"See https://encore.dev/docs/develop/api-schemas#supported-types for more information."),
	)
)
