package gcp

import (
	"context"
	"testing"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"encore.dev/metrics"
)

func TestExporter_Export(t *testing.T) {
	client, _ := monitoring.NewMetricClient(context.Background())
	x := &Exporter{
		cfg:              &Config{ProjectID: "encore-andre-dev-apps"},
		client:           client,
		firstSeenCounter: make(map[uint64]*timestamppb.Timestamp),
	}

	m := []metrics.CollectedMetric{{
		MetricName:   "andre_test",
		Type:         metrics.CounterType,
		Val:          int64(123),
		TimeSeriesID: 1,
		Labels:       []metrics.KeyValue{{Key: "encore", Value: "test"}},
	}}
	if err := x.Export(context.Background(), m); err != nil {
		t.Fatalf("export err: %v", err)
	}
}
