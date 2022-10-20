package custommetrics

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
)

func TestCounter_AWS(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	awsMetricsManager := awsMetricsManager{
		metricPrefix: "app_a1b2",
		logger:       logger,
	}

	awsMetricsManager.Counter("test_counter", map[string]string{"key": "value"})
	actual := buf.String()
	want := `{"level":"trace","e_metric_name":"app_a1b2_test_counter","key":"value"}
`
	if actual != want {
		t.Fatalf("\nwant:\n\t%q\ngot:\n\t%q\n", want, actual)
	}
}
