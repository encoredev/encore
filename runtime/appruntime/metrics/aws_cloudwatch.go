package metrics

import (
	"github.com/rs/zerolog"
)

// CloudWatch logs-based metrics support up to three dimensions:
//
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html#logs-metric-filters-dimensions
//
// For this reason, we check that the number of tags passed in by the caller
// isn't greater than three. This doesn't cover the case where the same metric is
// published with different sets of tags over multiple calls to the functions
// defined in this file.

type AWSMetricsExporter struct {
	logger zerolog.Logger
}

func NewAWSMetricsExporter(logger zerolog.Logger) *AWSMetricsExporter {
	return &AWSMetricsExporter{logger: logger}
}

func (e *AWSMetricsExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	logCounter(e.logger, name, tags...)
}

func (e *AWSMetricsExporter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	logValue(e.logger, name, key, value, tags...)
}
