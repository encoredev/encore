package metricstest

import (
	"reflect"
	"testing"

	"github.com/rs/zerolog"
)

// TestMetricsExporter is meant to be used in tests. It mimics the behavior of
// other metrics exporters in production in that it panics when the caller passes
// in a metric with more than three dimensions.
//
// Also, it keeps track of which metrics have been exported. This is useful in
// tests when you want to assert that the code under test has emitted metrics.
type TestMetricsExporter struct {
	logger          zerolog.Logger
	ExportedMetrics []*ExportedMetric
}

type MetricType string

const (
	MetricTypeCounter     MetricType = "counter"
	MetricTypeObservation MetricType = "observation"
)

type ExportedMetric struct {
	metricType MetricType
	name       string
	key        string // Only populated for 'observation' metric types
	value      float64
	tags       map[string]string
}

func NewTestMetricsExporter(logger zerolog.Logger) *TestMetricsExporter {
	return &TestMetricsExporter{logger: logger}
}

func (e *TestMetricsExporter) IncCounter(name string, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	tagMap := tagMap(tags...)
	metric := e.find(MetricTypeCounter, name, tagMap)
	if metric != nil {
		metric.value += 1
		return
	}

	e.ExportedMetrics = append(e.ExportedMetrics, &ExportedMetric{
		metricType: MetricTypeCounter,
		name:       name,
		value:      1,
		tags:       tagMap,
	})
}

func (e *TestMetricsExporter) Observe(name string, key string, value float64, tags ...string) {
	// See comment above.
	if len(tags) > 6 {
		panic("emitting metric with more than 3 dimensions is not supported")
	}

	tagMap := tagMap(tags...)
	e.ExportedMetrics = append(e.ExportedMetrics, &ExportedMetric{
		metricType: MetricTypeObservation,
		name:       name,
		key:        key,
		value:      value,
		tags:       tagMap,
	})
}

func tagMap(tags ...string) map[string]string {
	tagMap := make(map[string]string, len(tags))
	for i := 0; i < len(tags); i += 2 {
		if i+1 < len(tags) {
			tagMap[tags[i]] = tags[i+1]
		}
	}
	return tagMap
}

func (e *TestMetricsExporter) AssertCounter(t *testing.T, name string, value int, tags map[string]string) {
	t.Helper()

	actual := e.find(MetricTypeCounter, name, tags)
	if actual == nil {
		t.Errorf("counter assertion failed: counter metric '%s' with tags %s not emitted", name, tags)
		return
	}

	if int(actual.value) != value {
		t.Errorf("counter assertion failed: expected counter value %d, got %d", value, int(actual.value))
	}
	return
}

func (e *TestMetricsExporter) AssertObservation(
	t *testing.T,
	name string,
	key string,
	assert func(value float64) bool,
	tags map[string]string,
) {
	t.Helper()

	actual := e.find(MetricTypeObservation, name, tags)
	if actual == nil {
		t.Errorf("observation assertion failed: observation metric '%s' with tags %s not emitted", name, tags)
		return
	}

	if actual.key != key {
		t.Errorf("observation assertion failed: expected %s, got %s", key, actual.key)
	}

	if !assert(actual.value) {
		t.Errorf("observation assertion failed: unexpected metric value %f", actual.value)
	}
	return
}

func (e *TestMetricsExporter) find(metricType MetricType, metricName string, tags map[string]string) *ExportedMetric {
	for _, metric := range e.ExportedMetrics {
		if metric.metricType == metricType &&
			metric.name == metricName &&
			reflect.DeepEqual(metric.tags, tags) {
			return metric
		}
	}
	return nil
}
