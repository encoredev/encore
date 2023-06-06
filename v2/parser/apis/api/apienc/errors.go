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
		"Anonymous fields in top-leve request/response types are not support.",
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

	ErrUnexportedFieldsNotSupported = errRange.New(
		"Invalid Auth data schema",
		"Unexported fields are not supported anywhere within an auth data.",
		errors.WithDetails(
			"Encore does not support unexported fields in auth data to ensure that "+
				"the full data structure is serializable between services. This prevents an unexported "+
				"zero value from being misinterpreted as a valid value and misinterpreted.",
		),
	)
)
