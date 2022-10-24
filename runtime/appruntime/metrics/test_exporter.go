package metrics

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

// TestMetricsExporter is meant to be used in tests. It mimics the behavior of
// other metrics exporter in production in that it prefixes metric names with the
// app slug and it panics when the caller passes in a metric with more than three
// dimensions.
type TestMetricsExporter struct {
	metricPrefix string
	logger       zerolog.Logger
}

func NewTestMetricsExporter(appSlug string, logger zerolog.Logger) *TestMetricsExporter {
	metricPrefix := strings.Replace(appSlug, "-", "_", 1)
	return &TestMetricsExporter{
		metricPrefix: metricPrefix,
		logger:       logger,
	}
}

func (e *TestMetricsExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	if e.metricPrefix != "" {
		name = fmt.Sprintf("%s_%s", e.metricPrefix, name)
	}
	logCounter(e.logger, name, tags...)
}

func (e *TestMetricsExporter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	if e.metricPrefix != "" {
		name = fmt.Sprintf("%s_%s", e.metricPrefix, name)
	}
	logValue(e.logger, name, key, value, tags...)
}
