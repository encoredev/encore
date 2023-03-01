//go:build !encore_no_gcp

package gcp

import (
	"context"
	"fmt"
	"math"
	gometrics "runtime/metrics"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/rs/zerolog"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/system"
	"encore.dev/internal/nativehist"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.GCPCloudMonitoringProvider, meta *metadata.ContainerMetadata, rootLogger zerolog.Logger) *Exporter {
	// Precompute container metadata labels.
	return &Exporter{
		svcs: svcs,
		cfg:  cfg,
		containerMetadataLabels: map[string]string{
			"service_id":  meta.ServiceID,
			"revision_id": meta.RevisionID,
			"instance_id": meta.InstanceID,
		},
		rootLogger: rootLogger,

		firstSeenCounter:    make(map[uint64]*timestamppb.Timestamp),
		firstSeenCounterSys: make(map[string]*timestamppb.Timestamp),

		metricNames: cfg.MetricNames,
	}
}

type Exporter struct {
	svcs                    []string
	cfg                     *config.GCPCloudMonitoringProvider
	containerMetadataLabels map[string]string
	rootLogger              zerolog.Logger

	clientMu sync.Mutex
	client   *monitoring.MetricClient

	firstSeenCounter    map[uint64]*timestamppb.Timestamp
	firstSeenCounterSys map[string]*timestamppb.Timestamp

	dummyStart, dummyEnd time.Time

	metricNames map[string]string
}

func (x *Exporter) Shutdown(force context.Context) {
	x.clientMu.Lock()
	defer x.clientMu.Unlock()
	if x.client != nil {
		_ = x.client.Close()
	}
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	// Call time.Now twice so we don't get identical timestamps,
	// which is not allowed for cumulative metrics.
	newCounterStart := time.Now().Add(-time.Microsecond)
	endTime := time.Now()

	data := x.getMetricData(newCounterStart, endTime, collected)
	data = append(data, x.getSysMetrics(endTime)...)
	if len(data) == 0 {
		return nil
	}

	err := x.getClient().CreateTimeSeries(ctx, &monitoringpb.CreateTimeSeriesRequest{
		Name:       "projects/" + x.cfg.ProjectID,
		TimeSeries: data,
	})
	if err != nil {
		return fmt.Errorf("write metrics to GCP Cloud Monitoring: %v", err)
	}
	return nil
}

func (x *Exporter) getMetricData(newCounterStart, endTime time.Time, collected []metrics.CollectedMetric) []*monitoringpb.TimeSeries {
	pbNewCounterStart := timestamppb.New(newCounterStart)
	pbEndTime := timestamppb.New(endTime)

	monitoredResource := &monitoredrespb.MonitoredResource{
		Type:   x.cfg.MonitoredResourceType,
		Labels: x.cfg.MonitoredResourceLabels,
	}

	data := make([]*monitoringpb.TimeSeries, 0, len(collected))
	for _, m := range collected {
		baseLabels := make(map[string]string, len(x.containerMetadataLabels)+len(m.Labels))
		for k, v := range x.containerMetadataLabels {
			baseLabels[k] = v
		}
		for _, v := range m.Labels {
			baseLabels[v.Key] = v.Value
		}

		var kind metricpb.MetricDescriptor_MetricKind
		interval := &monitoringpb.TimeInterval{EndTime: pbEndTime}
		switch m.Info.Type() {
		case metrics.CounterType:
			// Determine when we first saw this time series.
			startTime := x.firstSeenCounter[m.TimeSeriesID]
			if startTime == nil {
				startTime = pbNewCounterStart
				x.firstSeenCounter[m.TimeSeriesID] = startTime
			}
			interval.StartTime = startTime

			kind = metricpb.MetricDescriptor_CUMULATIVE
		case metrics.GaugeType:
			kind = metricpb.MetricDescriptor_GAUGE
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown metric type %v for metric %s", m.Info.Type(), m.Info.Name())
			continue
		}

		svcNum := m.Info.SvcNum()
		metricType := "custom.googleapis.com/" + m.Info.Name()
		cloudMetricName, ok := x.metricNames[m.Info.Name()]
		if !ok {
			x.rootLogger.Error().Msgf("encore: internal error: metric %s not found in config", m.Info.Name())
			continue
		}
		metricType = "custom.googleapis.com/" + cloudMetricName

		doAdd := func(val *monitoringpb.TypedValue, svcIdx uint16) {
			labels := make(map[string]string, len(baseLabels)+1)
			for k, v := range baseLabels {
				labels[k] = v
			}
			labels["service"] = x.svcs[svcIdx]

			data = append(data, &monitoringpb.TimeSeries{
				MetricKind: kind,
				Metric: &metricpb.Metric{
					Type:   metricType,
					Labels: labels,
				},
				Resource: monitoredResource,
				Points: []*monitoringpb.Point{{
					Interval: interval,
					Value:    val,
				}},
			})
		}

		switch vals := m.Val.(type) {
		case []float64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(floatVal(vals[0]), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(floatVal(val), uint16(i))
					}
				}
			}

		case []int64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(int64Val(vals[0]), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(int64Val(val), uint16(i))
					}
				}
			}

		case []uint64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(uint64Val(vals[0]), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(uint64Val(val), uint16(i))
					}
				}
			}

		case []time.Duration:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(floatVal(float64(vals[0]/time.Second)), svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(floatVal(float64(val/time.Second)), uint16(i))
					}
				}
			}

		case []*nativehist.Histogram:
			// TODO implement support

		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s",
				m.Val, m.Info.Name())
		}

	}

	return data
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

	switch m.Info.Type() {
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
		panic(fmt.Sprintf("unhandled metric type %v", m.Info.Type()))
	}
	return point, kind
}

func floatVal(val float64) *monitoringpb.TypedValue {
	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DoubleValue{
			DoubleValue: val,
		},
	}
}

func int64Val(val int64) *monitoringpb.TypedValue {
	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_Int64Value{
			Int64Value: val,
		},
	}
}
func uint64Val(val uint64) *monitoringpb.TypedValue {
	// Return a float if this value exceeds the range of int64.
	if val > math.MaxInt64 {
		return &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: float64(val),
			},
		}
	}
	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_Int64Value{
			Int64Value: int64(val),
		},
	}
}

func (x *Exporter) getSysMetrics(now time.Time) []*monitoringpb.TimeSeries {
	monitoredResource := &monitoredrespb.MonitoredResource{
		Type:   x.cfg.MonitoredResourceType,
		Labels: x.cfg.MonitoredResourceLabels,
	}
	sysMetrics := system.ReadSysMetrics(x.rootLogger)
	output := make([]*monitoringpb.TimeSeries, 0, len(sysMetrics))
	for _, sysMetric := range sysMetrics {
		cloudMetricName, ok := x.metricNames[sysMetric.EncoreName]
		if !ok {
			x.rootLogger.Error().Msgf("encore: internal error: metric %s not found in config", sysMetric.EncoreName)
			continue
		}

		interval := &monitoringpb.TimeInterval{EndTime: timestamppb.New(now)}
		var metricKind metricpb.MetricDescriptor_MetricKind
		switch sysMetric.Kind {
		case system.MetricKindCounter:
			metricKind = metricpb.MetricDescriptor_CUMULATIVE
			startTime := x.firstSeenCounterSys[sysMetric.EncoreName]
			if startTime == nil {
				startTime = timestamppb.New(now.Add(-time.Microsecond))
				x.firstSeenCounterSys[sysMetric.EncoreName] = startTime
			}
			interval.StartTime = startTime
		case system.MetricKindGauge:
			metricKind = metricpb.MetricDescriptor_GAUGE
		default:
			x.rootLogger.Error().Str("metric_name", sysMetric.EncoreName).Msg("encore: internal error: unexpected system metric kind")
			continue
		}

		switch sysMetric.Sample.Value.Kind() {
		case gometrics.KindUint64:
			output = append(output, &monitoringpb.TimeSeries{
				MetricKind: metricKind,
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/" + cloudMetricName,
					Labels: x.containerMetadataLabels,
				},
				Resource: monitoredResource,
				Points: []*monitoringpb.Point{{
					Interval: interval,
					Value:    uint64Val(sysMetric.Sample.Value.Uint64()),
				}},
			})
		case gometrics.KindFloat64:
			output = append(output, &monitoringpb.TimeSeries{
				MetricKind: metricKind,
				Metric: &metricpb.Metric{
					Type:   "custom.googleapis.com/" + cloudMetricName,
					Labels: x.containerMetadataLabels,
				},
				Resource: monitoredResource,
				Points: []*monitoringpb.Point{{
					Interval: interval,
					Value:    floatVal(sysMetric.Sample.Value.Float64()),
				}},
			})
		default:
			x.rootLogger.Warn().Str("metric_name", sysMetric.Sample.Name).Msg("internal: unexpected metric kind")
			continue
		}
	}
	return output
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
