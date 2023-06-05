//go:build !encore_no_aws

package aws

import (
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/testing/protocmp"

	"encore.dev/appruntime/infrasdk/metadata"
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
	meta := &metadata.ContainerMetadata{
		ServiceID:  "a-fargate-task",
		RevisionID: "43",
		InstanceID: "a-fargate-task-instance",
	}

	baseDimensions := []types.Dimension{
		{Name: aws.String("service_id"), Value: aws.String("a-fargate-task")},
		{Name: aws.String("revision_id"), Value: aws.String("43")},
		{Name: aws.String("instance_id"), Value: aws.String("a-fargate-task-instance")},
	}

	makeDimensions := func(dims ...types.Dimension) []types.Dimension {
		rtn := make([]types.Dimension, len(baseDimensions)+len(dims))
		i := 0
		for _, dim := range baseDimensions {
			rtn[i] = dim
			i++
		}
		for _, dim := range dims {
			rtn[i] = dim
			i++
		}
		return rtn
	}

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
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: []types.MetricDatum{{
				MetricName: aws.String("test_counter"),
				Dimensions: makeDimensions(
					types.Dimension{Name: aws.String("service"), Value: aws.String("foo")},
				),
				Timestamp: aws.Time(now),
				Value:     aws.Float64(10),
			}},
		},
		{
			name: "gauge",
			metric: metrics.CollectedMetric{
				Info: metricInfo{"test_gauge", metrics.GaugeType, 2},
				Val:  []float64{0.5},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: []types.MetricDatum{{
				MetricName: aws.String("test_gauge"),
				Dimensions: makeDimensions(
					types.Dimension{Name: aws.String("service"), Value: aws.String("bar")},
				),
				Timestamp: aws.Time(now),
				Value:     aws.Float64(0.5),
			}},
		},
		{
			name: "labels",
			metric: metrics.CollectedMetric{
				Info:   metricInfo{"test_labels", metrics.GaugeType, 1},
				Labels: []metrics.KeyValue{{"key", "value"}},
				Val:    []float64{-1.5},
				Valid: func() []atomic.Bool {
					valid := make([]atomic.Bool, 1)
					valid[0].Store(true)
					return valid
				}(),
			},
			data: []types.MetricDatum{{
				MetricName: aws.String("test_labels"),
				Dimensions: makeDimensions(
					types.Dimension{Name: aws.String("key"), Value: aws.String("value")},
					types.Dimension{Name: aws.String("service"), Value: aws.String("foo")},
				),
				Timestamp: aws.Time(now),
				Value:     aws.Float64(-1.5),
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			x := New(svcs, nil, meta, zerolog.New(io.Discard))

			got := x.getMetricData(now, []metrics.CollectedMetric{test.metric})
			if diff := cmp.Diff(
				got, test.data,
				protocmp.Transform(),
				cmpopts.IgnoreUnexported(types.Dimension{}, types.MetricDatum{}),
			); diff != "" {
				t.Errorf("getMetricData() mismatch (-got +want):\n%s", diff)
			}
		})
	}
}
