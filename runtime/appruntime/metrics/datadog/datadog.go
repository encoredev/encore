package datadog

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/system"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.DatadogProvider, meta *metadata.ContainerMetadata, rootLogger zerolog.Logger) *Exporter {
	// Precompute container metadata labels.
	return &Exporter{
		svcs: svcs,
		cfg:  cfg,
		containerMetadataLabels: []string{
			"service_id:" + meta.ServiceID,
			"revision_id:" + meta.RevisionID,
			"instance_id:" + meta.InstanceID,
		},
		rootLogger: rootLogger,
	}
}

type Exporter struct {
	svcs                    []string
	cfg                     *config.DatadogProvider
	containerMetadataLabels []string
	rootLogger              zerolog.Logger
}

func (x *Exporter) Shutdown(_ context.Context) {}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now()
	data := x.getMetricData(now, collected)
	data = append(data, x.getSysMetrics(now)...)
	body := datadogV2.MetricPayload{Series: data}

	ctx = x.newContext(ctx)
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)
	api := datadogV2.NewMetricsApi(apiClient)
	_, _, err := api.SubmitMetrics(ctx, body, *datadogV2.NewSubmitMetricsOptionalParameters())
	if err != nil {
		return fmt.Errorf("unable to send metrics to Datadog: %v", err)
	}
	return nil
}

func (x *Exporter) getMetricData(now time.Time, collected []metrics.CollectedMetric) []datadogV2.MetricSeries {
	data := make([]datadogV2.MetricSeries, 0, len(collected))
	for _, m := range collected {
		var metricType *datadogV2.MetricIntakeType
		switch m.Info.Type() {
		case metrics.CounterType:
			metricType = datadogV2.METRICINTAKETYPE_COUNT.Ptr()
		case metrics.GaugeType:
			metricType = datadogV2.METRICINTAKETYPE_GAUGE.Ptr()
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown metric type %v for metric %s", m.Info.Type(), m.Info.Name())
			continue
		}

		labels := make([]string, len(x.containerMetadataLabels))
		copy(labels, x.containerMetadataLabels)
		for _, label := range m.Labels {
			labels = append(labels, label.Key+":"+label.Value)
		}

		doAdd := func(val float64, metricName string, baseLabels []string, svcIdx uint16) {
			labels := make([]string, len(baseLabels)+1)
			copy(labels, baseLabels)
			labels[len(baseLabels)] = "service:" + x.svcs[svcIdx]
			data = append(data, datadogV2.MetricSeries{
				Interval: datadog.PtrInt64(0),
				Metric:   metricName,
				Points: []datadogV2.MetricPoint{{
					Timestamp: datadog.PtrInt64(now.Unix()),
					Value:     datadog.PtrFloat64(val),
				}},
				Tags: labels,
				Type: metricType,
			})
		}

		svcNum := m.Info.SvcNum()
		switch vals := m.Val.(type) {
		case []float64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(vals[0], m.Info.Name(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(val, m.Info.Name(), labels, uint16(i))
					}
				}
			}
		case []int64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]), m.Info.Name(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val), m.Info.Name(), labels, uint16(i))
					}
				}
			}
		case []uint64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]), m.Info.Name(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val), m.Info.Name(), labels, uint16(i))
					}
				}
			}
		case []time.Duration:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]/time.Second), m.Info.Name(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val/time.Second), m.Info.Name(), labels, uint16(i))
					}
				}
			}
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s", m.Val, m.Info.Name())
		}
	}

	return data
}

func (x *Exporter) getSysMetrics(now time.Time) []datadogV2.MetricSeries {
	sysMetrics := system.ReadSysMetrics(x.rootLogger)
	return []datadogV2.MetricSeries{
		{
			Metric: system.MetricNameHeapObjectsBytes,
			Points: []datadogV2.MetricPoint{{
				Timestamp: datadog.PtrInt64(now.Unix()),
				Value:     datadog.PtrFloat64(float64(sysMetrics[system.MetricNameHeapObjectsBytes])),
			}},
			Tags: x.containerMetadataLabels,
			Type: datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
		},
		{
			Metric: system.MetricNameGoroutines,
			Points: []datadogV2.MetricPoint{{
				Timestamp: datadog.PtrInt64(now.Unix()),
				Value:     datadog.PtrFloat64(float64(sysMetrics[system.MetricNameGoroutines])),
			}},
			Tags: x.containerMetadataLabels,
			Type: datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
		},
	}
}

func (x *Exporter) newContext(parent context.Context) context.Context {
	return context.WithValue(
		context.WithValue(
			parent,
			datadog.ContextServerVariables,
			map[string]string{"site": x.cfg.Site},
		),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {Key: x.cfg.APIKey},
		},
	)
}
