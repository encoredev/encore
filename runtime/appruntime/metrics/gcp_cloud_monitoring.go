package metrics

import (
	"fmt"
	"strings"

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
	metricPrefix string
	logger       zerolog.Logger
}

func NewGCPMetricsExporter(appSlug string, logger zerolog.Logger) *GCPMetricsExporter {
	metricPrefix := strings.Replace(appSlug, "-", "_", 1)
	return &GCPMetricsExporter{
		metricPrefix: metricPrefix,
		logger:       logger,
	}
}

func (e *GCPMetricsExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting counter metric with more than 3 dimensions is not supported")
	}

	name = fmt.Sprintf("%s_%s", e.metricPrefix, name)
	logCounter(e.logger, name, tags...)
}
