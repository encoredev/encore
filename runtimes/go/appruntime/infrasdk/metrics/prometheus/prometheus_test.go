package prometheus

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"encore.dev/appruntime/infrasdk/metrics/prometheus/prompb"
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
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
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
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
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
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: []*prompb.TimeSeries{
				{
					Labels: []*prompb.Label{
						{
							Name:  "__name__",
							Value: "test_labels",
						},
						{
							Name:  "key",
							Value: "value",
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
							Value:     1,
							Timestamp: FromTime(now),
						},
					},
				},
				{
					Labels: []*prompb.Label{
						{
							Name:  "__name__",
							Value: "test_counter",
						},
						{
							Name:  "service",
							Value: "bar",
						},
					},
					Samples: []*prompb.Sample{
						{
							Value:     1,
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

func TestGetMetricData_ServiceLabels(t *testing.T) {
	now := time.Now()
	svcs := []string{"svc_a", "svc_b"}

	svcLabels := map[string][]metrics.KeyValue{
		"svc_a": {{Key: "team", Value: "backend"}},
	}

	t.Run("service labels included in output", func(t *testing.T) {
		m := metrics.CollectedMetric{
			Info:          metricInfo{"test_counter", metrics.CounterType, 1},
			Val:           []int64{5},
			ServiceLabels: svcLabels,
			Valid: func() []atomic.Bool {
				v := make([]atomic.Bool, 1)
				v[0].Store(true)
				return v
			}(),
		}

		x := &Exporter{svcs: svcs}
		got := x.getMetricData(now, []metrics.CollectedMetric{m})

		if len(got) != 1 {
			t.Fatalf("got %d items, want 1", len(got))
		}

		want := []*prompb.Label{
			{Name: "__name__", Value: "test_counter"},
			{Name: "service", Value: "svc_a"},
			{Name: "team", Value: "backend"},
		}
		if !reflect.DeepEqual(got[0].Labels, want) {
			t.Errorf("got labels %+v, want %+v", got[0].Labels, want)
		}
	})

	t.Run("no service labels for unregistered service", func(t *testing.T) {
		m := metrics.CollectedMetric{
			Info:          metricInfo{"test_counter", metrics.CounterType, 2},
			Val:           []int64{5},
			ServiceLabels: svcLabels,
			Valid: func() []atomic.Bool {
				v := make([]atomic.Bool, 1)
				v[0].Store(true)
				return v
			}(),
		}

		x := &Exporter{svcs: svcs}
		got := x.getMetricData(now, []metrics.CollectedMetric{m})

		if len(got) != 1 {
			t.Fatalf("got %d items, want 1", len(got))
		}

		want := []*prompb.Label{
			{Name: "__name__", Value: "test_counter"},
			{Name: "service", Value: "svc_b"},
		}
		if !reflect.DeepEqual(got[0].Labels, want) {
			t.Errorf("got labels %+v, want %+v", got[0].Labels, want)
		}
	})

	t.Run("multi-service metric with service labels", func(t *testing.T) {
		m := metrics.CollectedMetric{
			Info:          metricInfo{"test_counter", metrics.CounterType, 0},
			Val:           []int64{1, 2},
			ServiceLabels: svcLabels,
			Valid: func() []atomic.Bool {
				v := make([]atomic.Bool, 2)
				v[0].Store(true)
				v[1].Store(true)
				return v
			}(),
		}

		x := &Exporter{svcs: svcs}
		got := x.getMetricData(now, []metrics.CollectedMetric{m})

		if len(got) != 2 {
			t.Fatalf("got %d items, want 2", len(got))
		}

		// svc_a has service labels
		wantA := []*prompb.Label{
			{Name: "__name__", Value: "test_counter"},
			{Name: "service", Value: "svc_a"},
			{Name: "team", Value: "backend"},
		}
		if !reflect.DeepEqual(got[0].Labels, wantA) {
			t.Errorf("svc_a: got labels %+v, want %+v", got[0].Labels, wantA)
		}

		// svc_b does not
		wantB := []*prompb.Label{
			{Name: "__name__", Value: "test_counter"},
			{Name: "service", Value: "svc_b"},
		}
		if !reflect.DeepEqual(got[1].Labels, wantB) {
			t.Errorf("svc_b: got labels %+v, want %+v", got[1].Labels, wantB)
		}
	})
}
