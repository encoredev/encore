package metrics

import (
	"github.com/rs/zerolog"

	"encore.dev/rlog"
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

const encoreMetricKey = rlog.InternalKeyPrefix + "metric_name"

type logsBasedEmitter struct {
	logger zerolog.Logger
}

func newLogsBasedEmitter(logger zerolog.Logger) *logsBasedEmitter {
	return &logsBasedEmitter{logger: logger}
}

func (e *logsBasedEmitter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	e.logCounter(name, tags...)
}

func (e *logsBasedEmitter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	e.logValue(name, key, value, tags...)
}

func (e *logsBasedEmitter) logCounter(name string, tags ...string) {
	ev := e.logger.Trace().Str(encoreMetricKey, name)
	ev = addTags(ev, tags...)
	ev.Send()
}

func (e *logsBasedEmitter) logValue(name string, observationKey string, observationValue float64, tags ...string) {
	ev := e.logger.Trace().Str(encoreMetricKey, name).Float64(observationKey, observationValue)
	ev = addTags(ev, tags...)
	ev.Send()
}

func addTags(ev *zerolog.Event, tags ...string) *zerolog.Event {
	for i := 0; i < len(tags); i += 2 {
		if i+1 < len(tags) {
			ev = ev.Str(tags[i], tags[i+1])
		}
	}
	return ev
}
