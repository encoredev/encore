package metrics

import (
	"github.com/rs/zerolog"
)

// TestMetricsExporter is meant to be used in tests. It mimics the behavior of
// other metrics exporters in production in that it panics when the caller passes
// in a metric with more than three dimensions.
type TestMetricsExporter struct {
	logger zerolog.Logger
}

func NewTestMetricsExporter(logger zerolog.Logger) *TestMetricsExporter {
	return &TestMetricsExporter{logger: logger}
}

func (e *TestMetricsExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	logCounter(e.logger, name, tags...)
}

func (e *TestMetricsExporter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	logValue(e.logger, name, key, value, tags...)
}
