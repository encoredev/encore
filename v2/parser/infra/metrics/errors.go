package metrics

import (
	"encr.dev/pkg/errors"
)

var (
	errRange = errors.Range(
		"metrics",
		"For more information on metrics, see https://encore.dev/docs/observability/metrics",
		errors.WithRangeSize(20),
	)

	errInvalidArgCount = errRange.Newf(
		"Invalid metric construction",
		"%s requires 2 arguments; the metric name and the config object, got %d arguments.",
	)

	errInvalidMetricType = errRange.New(
		"Invalid metric construction",
		"The metric value type must be a builtin type.",
	)

	errInvalidLabelType = errRange.New(
		"Invalid metric construction",
		"The metric label type must be a named struct type.",
	)

	errLabelNoPointer = errRange.New(
		"Invalid metric construction",
		"The metric label type must not be a pointer.",
	)

	errLabelNoAnonymous = errRange.New(
		"Invalid metric label type",
		"Anonymous fields are not supported in metric labels.",
	)

	errLabelInvalidType = errRange.New(
		"Invalid metric label type",
		"Invalid metric label field: must be string, bool, or integer type.",
	)

	errLabelReservedName = errRange.New(
		"Invalid metric label name",
		"Metric labels cannot be named 'service' as this is reserved by Encore.",
	)
)
