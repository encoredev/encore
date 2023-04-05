//go:build !encore_no_gcp

package gcp

import (
	"io"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/google/go-cmp/cmp"
	"github.com/rs/zerolog"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredres "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/exported/config"
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
	newCounterStart := time.Now()
	now := time.Now()
	cfg := &config.GCPCloudMonitoringProvider{
		ProjectID:               "test-project",
		MonitoredResourceType:   "resource-type",
		MonitoredResourceLabels: map[string]string{"key": "value"},
	}
	monitoredRes := &monitoredres.MonitoredResource{
		Type:   "resource-type",
		Labels: map[string]string{"key": "value"},
	}
	pbStart := timestamppb.New(newCounterStart)
	pbEnd := timestamppb.New(now)

	svcs := []string{"foo", "bar"}
	tests := []struct {
		name   string
		metric metrics.CollectedMetric
		data   []*monitoringpb.TimeSeries
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
			data: []*monitoringpb.TimeSeries{{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/test_counter",
					Labels: map[string]string{"service": "foo"},
				},
				Resource:   monitoredRes,
				MetricKind: metricpb.MetricDescriptor_CUMULATIVE,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{StartTime: pbStart, EndTime: pbEnd},
					Value:    int64Val(10),
				}},
			}},
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
			data: []*monitoringpb.TimeSeries{{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/test_gauge",
					Labels: map[string]string{"service": "bar"},
				},
				Resource:   monitoredRes,
				MetricKind: metricpb.MetricDescriptor_GAUGE,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
					Value:    floatVal(0.5),
				}},
			}},
		},
		{
			name: "labels",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 1},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    []uint64{2},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: []*monitoringpb.TimeSeries{{
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/test_labels",
					Labels: map[string]string{"service": "foo", "key": "value"},
				},
				Resource:   monitoredRes,
				MetricKind: metricpb.MetricDescriptor_GAUGE,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
					Value:    int64Val(2),
				}},
			}},
		},
		{
			name: "labels_multi_svcs",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 0},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    []time.Duration{2 * time.Second, 4 * time.Second},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 2)
					valid[0].Store(true)
					valid[1].Store(true)
					return valid
				}(),
			},
			data: []*monitoringpb.TimeSeries{
				{
					Metric: &metricpb.Metric{
						Type:   "custom.googleapis.com/test_labels",
						Labels: map[string]string{"service": "foo", "key": "value"},
					},
					Resource:   monitoredRes,
					MetricKind: metricpb.MetricDescriptor_GAUGE,
					Points: []*monitoringpb.Point{{
						Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
						Value:    floatVal(2),
					}},
				},
				{
					Metric: &metricpb.Metric{
						Type:   "custom.googleapis.com/test_labels",
						Labels: map[string]string{"service": "bar", "key": "value"},
					},
					Resource:   monitoredRes,
					MetricKind: metricpb.MetricDescriptor_GAUGE,
					Points: []*monitoringpb.Point{{
						Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
						Value:    floatVal(4),
					}},
				},
			},
		},
		{
			name: "invalid_counter",
			metric: metrics.CollectedMetric{
				Info:  metricInfo{"test_counter", metrics.CounterType, 1},
				Val:   make([]int64, 1),
				Valid: make([]atomic.Bool, 1),
			},
			data: []*monitoringpb.TimeSeries{},
		},
		{
			name: "unset_gauges",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_gauges", metrics.GaugeType, 0},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    []uint64{1, 0},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 2)
					valid[0].Store(true)
					valid[1].Store(false)
					return valid
				}(),
			},
			data: []*monitoringpb.TimeSeries{
				{
					Metric: &metricpb.Metric{
						Type:   "custom.googleapis.com/test_gauges",
						Labels: map[string]string{"service": "foo", "key": "value"},
					},
					Resource:   monitoredRes,
					MetricKind: metricpb.MetricDescriptor_GAUGE,
					Points: []*monitoringpb.Point{{
						Interval: &monitoringpb.TimeInterval{EndTime: pbEnd},
						Value:    uint64Val(1),
					}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg.MetricNames = map[string]string{
				test.metric.Info.Name(): test.metric.Info.Name(),
			}
			x := New(svcs, cfg, zerolog.New(io.Discard))
			got := x.getMetricData(newCounterStart, now, []metrics.CollectedMetric{test.metric})
			if diff := cmp.Diff(got, test.data, protocmp.Transform()); diff != "" {
				t.Errorf("getMetricData() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
