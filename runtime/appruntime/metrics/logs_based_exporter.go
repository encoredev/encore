package metrics

import (
	"github.com/rs/zerolog"
)

// AWS CloudWatch logs-based metrics support up to three dimensions:
//
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html#logs-metric-filters-dimensions
//
// For this reason, we check that the number of tags passed in by the caller
// isn't greater than three. This doesn't cover the case where the same metric is
// published with different sets of tags over multiple calls to the functions
// defined in this file.
//
// GCP Cloud Monitoring logs-based metrics support up to ten dimensions (referred
// to as 'labels'):
//
// https://cloud.google.com/logging/docs/logs-based-metrics/labels#limitations
//
// However, since logs-based metrics with more than three dimensions aren't
// supported by AWS CloudWatch, we support only up to three dimensions per metric
// in GCP too.

const (
	EncoreMetricKeyPrefix = "encore_"
	encoreMetricKey       = EncoreMetricKeyPrefix + "metric_name"
)

type LogsBasedExporter struct {
	logger zerolog.Logger
}

func NewLogsBasedExporter(logger zerolog.Logger) *LogsBasedExporter {
	return &LogsBasedExporter{logger: logger}
}

func (e *LogsBasedExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	e.logCounter(name, tags...)
}

func (e *LogsBasedExporter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	e.logValue(name, key, value, tags...)
}

func (e *LogsBasedExporter) logCounter(name string, tags ...string) {
	loggerCtx := e.logger.With().Str(encoreMetricKey, name)
	loggerCtx = addTags(loggerCtx, tags...)
	logger := loggerCtx.Logger()
	logger.Trace().Send()
}

func (e *LogsBasedExporter) logValue(name string, observationKey string, observationValue float64, tags ...string) {
	loggerCtx := e.logger.With().Str(encoreMetricKey, name).Float64(observationKey, observationValue)
	loggerCtx = addTags(loggerCtx, tags...)
	logger := loggerCtx.Logger()
	logger.Trace().Send()
}

func addTags(loggerCtx zerolog.Context, tags ...string) zerolog.Context {
	for i := 0; i < len(tags); i += 2 {
		if i+1 < len(tags) {
			loggerCtx = loggerCtx.Str(tags[i], tags[i+1])
		}
	}
	return loggerCtx
}
