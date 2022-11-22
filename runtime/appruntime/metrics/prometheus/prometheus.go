package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.PrometheusRemoteWriteProvider, rootLogger zerolog.Logger) *Exporter {
	return &Exporter{
		svcs: svcs,
		cfg:  cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		rootLogger: rootLogger,
	}
}

type Exporter struct {
	svcs       []string
	cfg        *config.PrometheusRemoteWriteProvider
	client     *http.Client
	rootLogger zerolog.Logger
}

func (x *Exporter) Shutdown(_ context.Context) {
}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	now := time.Now()
	data := x.getMetricData(now, collected)
	proto, err := proto.Marshal(&prompb.WriteRequest{Timeseries: data})
	if err != nil {
		return fmt.Errorf("unable to marshal metrics into Protobuf: %v", err)
	}

	encoded := snappy.Encode(nil, proto)
	body := bytes.NewReader(encoded)
	req, err := http.NewRequest(http.MethodPost, x.cfg.RemoteWriteURL, body)
	if err != nil {
		return fmt.Errorf("unable to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("User-Agent", "encore")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	_, err = x.client.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("unable to send metrics to Prometheus remote write destination: %v", err)
	}

	return nil
}

func (x *Exporter) getMetricData(now time.Time, collected []metrics.CollectedMetric) []prompb.TimeSeries {
	data := make([]prompb.TimeSeries, 0, len(collected))

	doAdd := func(val float64, metricName string, baseLabels []prompb.Label, svcIdx uint16) {
		labels := make([]prompb.Label, len(baseLabels)+2)
		copy(labels, baseLabels)
		labels[len(baseLabels)] = prompb.Label{
			Name:  "__name__",
			Value: metricName,
		}
		labels[len(baseLabels)+1] = prompb.Label{
			Name:  "service",
			Value: x.svcs[svcIdx],
		}
		data = append(data, prompb.TimeSeries{
			Labels: labels,
			Samples: []prompb.Sample{
				{
					Value:     val,
					Timestamp: timestamp.FromTime(now),
				},
			},
		})
	}

	for _, m := range collected {
		var labels []prompb.Label
		if n := len(m.Labels); n > 0 {
			labels = make([]prompb.Label, 0, n)
			for _, label := range m.Labels {
				labels = append(labels, prompb.Label{
					Name:  label.Key,
					Value: label.Value,
				})
			}
		}

		svcNum := m.Info.SvcNum()
		switch vals := m.Val.(type) {
		case []float64:
			if svcNum > 0 {
				doAdd(vals[0], m.Info.Name(), labels, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(val, m.Info.Name(), labels, uint16(i))
				}
			}
		case []int64:
			if svcNum > 0 {
				doAdd(float64(vals[0]), m.Info.Name(), labels, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(float64(val), m.Info.Name(), labels, uint16(i))
				}
			}
		case []uint64:
			if svcNum > 0 {
				doAdd(float64(vals[0]), m.Info.Name(), labels, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(float64(val), m.Info.Name(), labels, uint16(i))
				}
			}
		case []time.Duration:
			if svcNum > 0 {
				doAdd(float64(vals[0]/time.Second), m.Info.Name(), labels, svcNum-1)
			} else {
				for i, val := range vals {
					doAdd(float64(val/time.Second), m.Info.Name(), labels, uint16(i))
				}
			}
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s",
				m.Val, m.Info.Name())
		}
	}

	return data
}
