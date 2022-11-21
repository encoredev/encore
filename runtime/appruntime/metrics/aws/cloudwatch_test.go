//go:build !encore_no_aws

package aws

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

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
	now := time.Now()
	svcs := []string{"foo", "bar"}
	tests := []struct {
		name   string
		metric metrics.CollectedMetric
		data   []types.MetricDatum
	}{
		{
			name: "counter",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_counter", metrics.CounterType, 1},
				Val:  []int64{10},
			},
			data: []types.MetricDatum{{
				MetricName: aws.String("test_counter"),
				Dimensions: []types.Dimension{{Name: aws.String("service"), Value: aws.String("foo")}},
				Timestamp:  aws.Time(now),
				Value:      aws.Float64(10),
			}},
		},
		{
			name: "gauge",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_gauge", metrics.GaugeType, 2},
				Val:  []float64{0.5},
			},
			data: []types.MetricDatum{{
				MetricName: aws.String("test_gauge"),
				Dimensions: []types.Dimension{{Name: aws.String("service"), Value: aws.String("bar")}},
				Timestamp:  aws.Time(now),
				Value:      aws.Float64(0.5),
			}},
		},
		{
			name: "labels",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 1},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    []float64{-1.5},
			},
			data: []types.MetricDatum{{
				MetricName: aws.String("test_labels"),
				Dimensions: []types.Dimension{
					{Name: aws.String("key"), Value: aws.String("value")},
					{Name: aws.String("service"), Value: aws.String("foo")},
				},
				Timestamp: aws.Time(now),
				Value:     aws.Float64(-1.5),
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := &Exporter{svcs: svcs}
			got := x.getMetricData(now, []metrics.CollectedMetric{test.metric})
			if len(got) != len(test.data) {
				t.Errorf("got %d items, want %d", len(got), len(test.data))
			} else if !reflect.DeepEqual(got, test.data) {
				t.Errorf("got %+v, want %+v", got, test.data)
			}
		})
	}
}
