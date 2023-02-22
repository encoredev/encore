package json_based

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

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
		data   JSONMetrics
	}{
		{
			name: "counter",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_counter", metrics.CounterType, 1},
				Val:  []int64{10},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: JSONMetrics{
				Metrics: []JSONMetric{
					{
						Name: "test_counter",
						Type: "Counter",
						Labels: []Label{
							{
								"service": "foo",
							},
						},
						Value: float64(10),
					},
				},
			},
		},
		{
			name: "gauge",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_gauge", metrics.GaugeType, 2},
				Val:  []float64{0.5},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: JSONMetrics{
				Metrics: []JSONMetric{
					{
						Name: "test_gauge",
						Type: "Gauge",
						// Labels: []Label{},
						Labels: []Label{
							{
								"service": "bar",
							},
						},
						Value: 0.5,
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
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: JSONMetrics{
				Metrics: []JSONMetric{
					{
						Name: "test_labels",
						Type: "Gauge",
						Labels: []Label{
							{
								"key": "value",
							},
							{
								"service": "foo",
							},
						},
						Value: float64(-1.5),
					},
				},
			},
		},
		{
			name: "multiple services",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_counter", metrics.CounterType, 0},
				Val:  []int64{1, 1},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 2)
					valid[0].Store(true)
					valid[1].Store(true)
					return valid
				}(),
			},
			data: JSONMetrics{
				Metrics: []JSONMetric{
					{
						Name: "test_counter",
						Type: "Counter",
						Labels: []Label{
							{
								"service": "foo",
							},
						},
						Value: float64(1),
					},
					{
						Name: "test_counter",
						Type: "Counter",
						Labels: []Label{
							{
								"service": "bar",
							},
						},
						Value: float64(1),
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := &Exporter{svcs: svcs}
			got := x.GetMetricData(now, []metrics.CollectedMetric{test.metric})
			if !reflect.DeepEqual(got, test.data) {
				t.Errorf("got %+v, want %+v", got, test.data)
			}
		})
	}
}
