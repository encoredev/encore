package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/metadata"
	"encore.dev/appruntime/metrics/prometheus/prompb"
	"encore.dev/appruntime/metrics/system"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.PrometheusRemoteWriteProvider, meta *metadata.ContainerMetadata, rootLogger zerolog.Logger) *Exporter {
	return &Exporter{
		svcs:              svcs,
		cfg:               cfg,
		containerMetadata: meta,
		rootLogger:        rootLogger,
	}
}

type Exporter struct {
	svcs              []string
	cfg               *config.PrometheusRemoteWriteProvider
	containerMetadata *metadata.ContainerMetadata
	rootLogger        zerolog.Logger
}

func (x *Exporter) Shutdown(_ context.Context) {}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now()
	data := x.getMetricData(now, collected)
	data = append(data, x.getSysMetrics(now)...)
	proto, err := proto.Marshal(&prompb.WriteRequest{Timeseries: data})
	if err != nil {
		return fmt.Errorf("unable to marshal metrics into Protobuf: %v", err)
	}

	encoded := snappy.Encode(nil, proto)
	body := bytes.NewReader(encoded)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, x.cfg.RemoteWriteURL, body)
	if err != nil {
		return fmt.Errorf("unable to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("User-Agent", "encore")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to send metrics to Prometheus remote write destination: %v", err)
	}

	return nil
}

func (x *Exporter) getMetricData(now time.Time, collected []metrics.CollectedMetric) []*prompb.TimeSeries {
	data := make([]*prompb.TimeSeries, 0, len(collected))

	doAdd := func(val float64, metricName string, baseLabels []*prompb.Label, svcIdx uint16) {
		containerMetadataLabels := containerMetadataLabels(x.containerMetadata)
		labels := make([]*prompb.Label, len(baseLabels)+len(containerMetadataLabels)+2)
		copy(labels, baseLabels)
		labels = append(labels, append(containerMetadataLabels,
			&prompb.Label{
				Name:  "__name__",
				Value: metricName,
			},
			&prompb.Label{
				Name:  "service",
				Value: x.svcs[svcIdx],
			},
		)...)
		data = append(data, &prompb.TimeSeries{
			Labels: labels,
			Samples: []*prompb.Sample{
				{
					Value:     val,
					Timestamp: FromTime(now),
				},
			},
		})
	}

	for _, m := range collected {
		var labels []*prompb.Label
		if n := len(m.Labels); n > 0 {
			labels = make([]*prompb.Label, 0, n)
			for _, label := range m.Labels {
				labels = append(labels, &prompb.Label{
					Name:  label.Key,
					Value: label.Value,
				})
			}
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
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s",
				m.Val, m.Info.Name())
		}
	}

	return data
}

func (x *Exporter) getSysMetrics(now time.Time) []*prompb.TimeSeries {
	containerMetadataLabels := containerMetadataLabels(x.containerMetadata)
	sysMetrics := system.ReadSysMetrics()
	return []*prompb.TimeSeries{
		{
			Labels: append(containerMetadataLabels, &prompb.Label{
				Name:  "__name__",
				Value: system.MetricNameMemUsageBytes,
			}),
			Samples: []*prompb.Sample{{
				Value:     float64(sysMetrics[system.MetricNameMemUsageBytes]),
				Timestamp: FromTime(now),
			}},
		},
		{
			Labels: append(containerMetadataLabels, &prompb.Label{
				Name:  "__name__",
				Value: system.MetricNameNumGoroutines,
			}),
			Samples: []*prompb.Sample{{
				Value:     float64(sysMetrics[system.MetricNameNumGoroutines]),
				Timestamp: FromTime(now),
			}},
		},
	}
}

func containerMetadataLabels(meta *metadata.ContainerMetadata) []*prompb.Label {
	return []*prompb.Label{
		{
			Name:  "service_id",
			Value: meta.ServiceID,
		},
		{
			Name:  "revision_id",
			Value: meta.RevisionID,
		},
		{
			Name:  "instance_id",
			Value: meta.InstanceID,
		},
	}
}

// FromTime returns a new millisecond timestamp from a time.
func FromTime(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/int64(time.Millisecond)
}
