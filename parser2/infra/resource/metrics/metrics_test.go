package metrics

import (
	"testing"

	"encr.dev/parser2/infra/resource/resourcetest"
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
				Name: "name",
				Doc:  "Metric docs\n",
			},
		},
		{
			Name: "gauge",
			Code: `
// Metric docs
var x = metrics.NewGauge[int]("name", metrics.GaugeConfig{})
`,
			Want: &Metric{
				Name: "name",
				Doc:  "Metric docs\n",
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
				Name: "name",
				Doc:  "Metric docs\n",
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
				Name: "name",
				Doc:  "Metric docs\n",
			},
		},
	}

	resourcetest.Run(t, MetricParser, tests)
}
