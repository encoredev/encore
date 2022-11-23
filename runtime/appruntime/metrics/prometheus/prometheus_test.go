package prometheus

import (
	"reflect"
	"testing"
	"time"

	"encore.dev/appruntime/metrics/prometheus/prompb"
	"encore.dev/metrics"
)

type metricInfo struct {
	name   string
	typ    metrics.MetricType
	svcNum uint16
}

func (m metricInfo) Name() string             { return m.name }
func (m metricInfo) Type() metrics.MetricType { return m.typ }
func (m metricInfo) SvcNum() uint16           { return m.svcNum }

func TestGetMetricData(t *testing.T) {
	now := time.Now()
	svcs := []string{"foo", "bar"}
	tests := []struct {
		name   string
		metric metrics.CollectedMetric
		data   []*prompb.TimeSeries
	}{
		{
			name: "counter",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_counter", metrics.CounterType, 1},
				Val:  []int64{10},
			},
			data: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						{
							Name:  "__name__",
							Value: "test_counter",
						},
						{
							Name:  "service",
							Value: "foo",
						},
					},
					Samples: []*prompb.Sample{
						{
							Value:     10,
							Timestamp: FromTime(now),
						},
					},
				},
			},
		},
		{
			name: "gauge",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_gauge", metrics.GaugeType, 2},
				Val:  []float64{0.5},
			},
			data: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						{
							Name:  "__name__",
							Value: "test_gauge",
						},
						{
							Name:  "service",
							Value: "bar",
						},
					},
					Samples: []*prompb.Sample{
						{
							Value:     0.5,
							Timestamp: FromTime(now),
						},
					},
				},
			},
		},
		{
			name: "labels",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 1},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    []float64{-1.5},
			},
			data: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						{
							Name:  "key",
							Value: "value",
						},
						{
							Name:  "__name__",
							Value: "test_labels",
						},
						{
							Name:  "service",
							Value: "foo",
						},
					},
					Samples: []*prompb.Sample{
						{
							Value:     -1.5,
							Timestamp: FromTime(now),
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := &Exporter{svcs: svcs}
			got := x.getMetricData(now, []metrics.CollectedMetric{test.metric})
			if len(got) != len(test.data) {
				t.Errorf("got %d items, want %d", len(got), len(test.data))
			} else if !reflect.DeepEqual(got, test.data) {
				t.Errorf("got %+v, want %+v", got, test.data)
			}
		})
	}
}
