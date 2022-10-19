package custommetrics

import (
	"github.com/rs/zerolog"
)

// Cloud Monitoring logs-based metrics support up to ten dimensions (referred to
// as 'labels'):
//
// https://cloud.google.com/logging/docs/logs-based-metrics/labels#limitations
//
// For this reason, we check that the number of tags passed in by the caller
// isn't greater than ten. This doesn't cover the case where the same metric is
// published with different sets of tags over multiple calls to the functions
// defined in this file.

type gcpMetricsManager struct {
	logger zerolog.Logger
}

func (m *gcpMetricsManager) Counter(name string, tags map[string]string) {
	// See comment above.
	if len(tags) > 10 {
		m.logger.Trace().Str("dropped_metric_name", name).Msg("dropping metric")
		return
	}

	loggerCtx := m.logger.With().Str("metric_name", name)
	for k, v := range tags {
		loggerCtx = loggerCtx.Str(k, v)
	}
	logger := loggerCtx.Logger()
	logger.Trace().Send()
}
