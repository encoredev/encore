package protoparse

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"protoparse",
		"For more information on gRPC, see https://encore.dev/docs/develop/grpc",

		errors.WithRangeSize(20),
	)

	errInvalidProtoFile = errRange.Newf(
		"Invalid protobuf file",
		"Unable to parse the protobuf file '%s'.",
	)
)
