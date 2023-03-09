package servicestruct

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"servicestruct",
		"For more information on service structs, see https://encore.dev/docs/primitives/services-and-apis#service-structs",

		errors.WithRangeSize(20),
	)

	errInvalidDirective = errRange.New(
		"Invalid encore:service directive",
		"invalid encore:service directive.",
		errors.MarkAsInternalError(),
	)

	errInvalidDirectivePlacement = errRange.New(
		"Invalid encore:service directive",
		"encore:service directives must be placed on the declaration of a struct, not a group.",
	)

	errServiceStructMustNotBeGeneric = errRange.New(
		"Invalid service struct",
		"Service structs cannot be defined as generic types.",
	)

	errServiceInitCannotBeGeneric = errRange.New(
		"Invalid service init function",
		"Service init functions cannot be defined as generic functions.",
	)

	errServiceInitCannotHaveParams = errRange.New(
		"Invalid service init function",
		"Service init functions cannot have parameters.",
	)

	errServiceInitInvalidReturnType = errRange.Newf(
		"Invalid service init function",
		"Service init functions must return (*%s, error).",
	)

	ErrDuplicateServiceStructs = errRange.New(
		"Multiple service structs found",
		"Multiple service structs were found in the same service. Encore only allows one service struct to be defined per service.",
	)
)
