package grpcservice

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"grpcservice",
		"For more information on service structs, see https://encore.dev/docs/primitives/services-and-apis#service-structs",

		errors.WithRangeSize(20),
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
)
