package json_based

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/metrics"
)

func New(svcs []string, cfg *config.JSONBasedMetricsProvider, rootLogger zerolog.Logger) *Exporter {
	return &Exporter{
		svcs:       svcs,
		cfg:        cfg,
		rootLogger: rootLogger,
	}
}

type Exporter struct {
	svcs       []string
	cfg        *config.JSONBasedMetricsProvider
	rootLogger zerolog.Logger
}

// Can be easily marshaled into a reasonable JSON object
type JSONMetrics struct {
	Metrics []JSONMetric `json:"metrics"`
}

type Label map[string]string

type JSONMetric struct {
	Name   string  `json:"name"`
	Type   string  `json:"type"`
	Labels []Label `json:"labels"`
	Value  any     `json:"value"`
}

func (x *Exporter) Shutdown(_ context.Context) {}

func (x *Exporter) Export(ctx context.Context, collected []metrics.CollectedMetric) error {
	// This is a no-op since there's currently no need to export JSON metrics;
	// they're only emitted via an internal api endpoint in runtime/appruntime/api/encore_routes.go
	return nil
}

// Use to export a slice of CollectedMetric into a shape that's useful when rendering
// metrics into JSON. Marshalling the return value will result in nicely formatted JSON
func (x *Exporter) GetMetricData(collected []metrics.CollectedMetric) JSONMetrics {
	// This method is based largely on the prometheus.Exporter.getMetricData method.
	// It handles the complexities of multi-value metrics and adding labels.
	data := make([]JSONMetric, 0, len(collected))

	doAdd := func(val float64, metricName string, metricType metrics.MetricType, baseLabels []Label, svcIdx uint16) {
		labels := make([]Label, len(baseLabels))
		copy(labels, baseLabels)
		// If there are services for this exporter, add labels
		if len(x.svcs) > 0 {
			svcLabel := Label{"service": x.svcs[svcIdx]}
			labels = append(labels, svcLabel)
		}
		data = append(data, JSONMetric{
			Name:   metricName,
			Type:   metricType.Name(),
			Labels: labels,
			Value:  val,
		})
	}

	for _, m := range collected {
		var labels []Label
		if n := len(m.Labels); n > 0 {
			labels = make([]Label, 0, n)
			for _, label := range m.Labels {
				l := Label{}
				l[label.Key] = label.Value
				labels = append(labels, l)
			}
		}

		svcNum := m.Info.SvcNum()
		switch vals := m.Val.(type) {
		case []float64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(vals[0], m.Info.Name(), m.Info.Type(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(val, m.Info.Name(), m.Info.Type(), labels, uint16(i))
					}
				}
			}
		case []int64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]), m.Info.Name(), m.Info.Type(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val), m.Info.Name(), m.Info.Type(), labels, uint16(i))
					}
				}
			}
		case []uint64:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]), m.Info.Name(), m.Info.Type(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val), m.Info.Name(), m.Info.Type(), labels, uint16(i))
					}
				}
			}
		case []time.Duration:
			if svcNum > 0 {
				if m.Valid[0].Load() {
					doAdd(float64(vals[0]/time.Second), m.Info.Name(), m.Info.Type(), labels, svcNum-1)
				}
			} else {
				for i, val := range vals {
					if m.Valid[i].Load() {
						doAdd(float64(val/time.Second), m.Info.Name(), m.Info.Type(), labels, uint16(i))
					}
				}
			}
		default:
			x.rootLogger.Error().Msgf("encore: internal error: unknown value type %T for metric %s",
				m.Val, m.Info.Name())
		}
	}

	return JSONMetrics{Metrics: data}

}
