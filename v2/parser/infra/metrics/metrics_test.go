package metrics

import (
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"encr.dev/v2/internals/schema/schematest"
	"encr.dev/v2/parser/resource/resourcetest"
)

func TestParseMetrics(t *testing.T) {
	tests := []resourcetest.Case[*Metric]{
		{
			Name: "counter",
			Code: `
// Metric docs
var x = metrics.NewCounter[int]("name", metrics.CounterConfig{})
`,
			Want: &Metric{
				Name:      "name",
				Doc:       "Metric docs\n",
				ValueType: schematest.Int(),
				Type:      Counter,
			},
		},
		{
			Name: "gauge",
			Code: `
// Metric docs
var x = metrics.NewGauge[int]("name", metrics.GaugeConfig{})
`,
			Want: &Metric{
				Name:      "name",
				Doc:       "Metric docs\n",
				Type:      Gauge,
				ValueType: schematest.Int(),
			},
		},
		{
			Name: "counter_group",
			Code: `
// Metric docs
var x = metrics.NewCounterGroup[Labels, int]("name", metrics.CounterConfig{})

type Labels struct {
	ID string
}
`,
			Want: &Metric{
				Name:      "name",
				Doc:       "Metric docs\n",
				Type:      Counter,
				Labels:    []Label{{Key: "id", Type: schematest.String()}},
				ValueType: schematest.Int(),
			},
		},
		{
			Name: "gauge_group",
			Code: `
// Metric docs
var x = metrics.NewGaugeGroup[Labels, int]("name", metrics.GaugeConfig{})

type Labels struct {
	ID string
}
`,
			Want: &Metric{
				Name:      "name",
				Doc:       "Metric docs\n",
				Labels:    []Label{{Key: "id", Type: schematest.String()}},
				ValueType: schematest.Int(),
				Type:      Gauge,
			},
		},
	}

	resourcetest.Run(t, MetricParser, tests, cmpopts.IgnoreFields(Metric{}, "LabelType"))
}
