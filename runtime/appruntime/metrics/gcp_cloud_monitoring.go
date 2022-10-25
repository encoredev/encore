package metrics

import (
	"github.com/rs/zerolog"
)

// Cloud Monitoring logs-based metrics support up to ten dimensions (referred to
// as 'labels'):
//
// https://cloud.google.com/logging/docs/logs-based-metrics/labels#limitations
//
// However, since CloudWatch logs-based metrics support up to three dimensions
// only, the code panics if the caller passes in more than three dimensions.

type GCPMetricsExporter struct {
	logger zerolog.Logger
}

func NewGCPMetricsExporter(logger zerolog.Logger) *GCPMetricsExporter {
	return &GCPMetricsExporter{logger: logger}
}

func (e *GCPMetricsExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	logCounter(e.logger, name, tags...)
}

func (e *GCPMetricsExporter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	logValue(e.logger, name, key, value, tags...)
}
