//go:build !encore_no_gcp

package gcp

import (
	"context"
	"fmt"
	"sync"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/config"
	"encore.dev/metrics"
)

func New(cfg *config.GCPCloudMonitoringProvider) *Exporter {
	return &Exporter{
		cfg:              cfg,
		firstSeenCounter: make(map[uint64]*timestamppb.Timestamp),
	}
}

type Exporter struct {
	cfg *config.GCPCloudMonitoringProvider

	clientMu sync.Mutex
	client   *monitoring.MetricClient

	firstSeenCounter map[uint64]*timestamppb.Timestamp
}

func (x *Exporter) Shutdown(force context.Context) {
	x.clientMu.Lock()
	defer x.clientMu.Unlock()
	if x.client != nil {
		_ = x.client.Close()
	}
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	newCounterStart := timestamppb.Now()
	endTime := timestamppb.Now()

	timeSeries := make([]*monitoringpb.TimeSeries, 0, len(collected))
	for _, m := range collected {
		var labels map[string]string
		if len(m.Labels) > 0 {
			labels = make(map[string]string, len(m.Labels))
			for _, v := range m.Labels {
				labels[v.Key] = v.Value
			}
		}

		point, kind := x.getPoint(newCounterStart, endTime, &m)
		timeSeries = append(timeSeries, &monitoringpb.TimeSeries{
			MetricKind: kind,
			Metric: &metricpb.Metric{
				Type:   "custom.googleapis.com/" + m.MetricName,
				Labels: labels,
			},
			Resource: &monitoredrespb.MonitoredResource{
				Type:   x.cfg.MonitoredResourceType,
				Labels: x.cfg.MonitoredResourceLabels,
			},
			Points: []*monitoringpb.Point{point},
		})
	}

	// Writes time series data.
	err := x.getClient().CreateTimeSeries(ctx, &monitoringpb.CreateTimeSeriesRequest{
		Name:       "projects/" + x.cfg.ProjectID,
		TimeSeries: timeSeries,
	})
	if err != nil {
		return fmt.Errorf("write metrics to GCP Cloud Monitoring: %v", err)
	}
	return nil
}

func (x *Exporter) getPoint(newCounterStart, endTime *timestamppb.Timestamp, m *metrics.CollectedMetric) (point *monitoringpb.Point, kind metricpb.MetricDescriptor_MetricKind) {
	value := &monitoringpb.TypedValue{}
	switch v := m.Val.(type) {
	case float64:
		value.Value = &monitoringpb.TypedValue_DoubleValue{DoubleValue: v}
	case int64:
		value.Value = &monitoringpb.TypedValue_Int64Value{Int64Value: v}
	default:
		panic(fmt.Sprintf("unhandled value type %T", v))
	}

	switch m.Type {
	case metrics.CounterType:
		// Determine when we first saw this time series.
		startTime := x.firstSeenCounter[m.TimeSeriesID]
		if startTime == nil {
			startTime = newCounterStart
			x.firstSeenCounter[m.TimeSeriesID] = startTime
		}

		kind = metricpb.MetricDescriptor_CUMULATIVE
		point = &monitoringpb.Point{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   endTime,
			},
			Value: value,
		}

	case metrics.GaugeType:
		kind = metricpb.MetricDescriptor_GAUGE
		point = &monitoringpb.Point{
			Interval: &monitoringpb.TimeInterval{
				EndTime: endTime,
			},
			Value: value,
		}
	default:
		panic(fmt.Sprintf("unhandled metric type %v", m.Type))
	}
	return point, kind
}

func (x *Exporter) getClient() *monitoring.MetricClient {
	x.clientMu.Lock()
	defer x.clientMu.Unlock()
	if x.client == nil {
		cl, err := monitoring.NewMetricClient(context.Background())
		if err != nil {
			panic(fmt.Sprintf("failed to create metrics client: %s", err))
		}
		x.client = cl
	}
	return x.client
}
