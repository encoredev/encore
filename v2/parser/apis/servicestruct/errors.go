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

	errInvalidGRPCName = errRange.Newf(
		"Invalid gRPC service name",
		"The grpc field must be a valid, fully-qualified gRPC service name (got \"%s\").",
	)

	errNonLocalGRPCPath = errRange.Newf(
		"Invalid gRPC file path",
		"The grpc field must be a relative path from a protobuf include directory (got \"%s\").",
	)

	errGRPCServiceNotFound = errRange.Newf(
		"gRPC service not found",
		"The gRPC service \"%s\" cannot be found in the file \"%s\".",
	)

	errServiceInitInvalidReturnType = errRange.Newf(
		"Invalid service init function",
		"Service init functions must return (*%s, error).",
	)

	ErrDuplicateServiceStructs = errRange.New(
		"Multiple service structs found",
		"Multiple service structs were found in the same service. Encore only allows one service struct to be defined per service.",
	)

	ErrReceiverNotAServiceStruct = errRange.New(
		"Invalid service struct for API",
		"API endpoints defined as receiver functions must be defined on a service struct.",
	)

	ErrServiceStructReferencedInAnotherService = errRange.New(
		"Service struct referenced in another service",
		"Service structs cannot be referenced in other services. They can only be referenced in the service that defines them.",
	)
)
