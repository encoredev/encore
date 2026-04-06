//go:build !encore_no_azure

package azure

import (
	"encoding/json"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/infrasdk/metadata"
	"encore.dev/appruntime/infrasdk/metrics/system"
	"encore.dev/metrics"
)

// metricInfo is a test implementation of metrics.MetricInfo.
type metricInfo struct {
	name   string
	typ    metrics.MetricType
	svcNum uint16
}

func (m metricInfo) Name() string             { return m.name }
func (m metricInfo) Type() metrics.MetricType { return m.typ }
func (m metricInfo) SvcNum() uint16           { return m.svcNum }

// validBools returns a slice of atomic.Bool, all set to true.
func validBools(n int) []atomic.Bool {
	v := make([]atomic.Bool, n)
	for i := range v {
		v[i].Store(true)
	}
	return v
}

// newTestExporter creates an Exporter with no real Azure config, suitable for
// testing the pure batch-building logic.
func newTestExporter(svcs []string, meta *metadata.ContainerMetadata) *Exporter {
	return New(svcs, nil, meta, zerolog.New(io.Discard))
}

// ---- getMetricBatches tests -----------------------------------------------------------

func TestGetMetricBatches_Counter(t *testing.T) {
	now := time.Now()
	svcs := []string{"svc-a", "svc-b"}
	meta := &metadata.ContainerMetadata{
		ServiceID:  "rg-prod",
		InstanceID: "inst-1",
	}

	x := newTestExporter(svcs, meta)
	collected := []metrics.CollectedMetric{
		{
			Info:  metricInfo{"http_requests_total", metrics.CounterType, 1},
			Val:   []int64{42},
			Valid: validBools(1),
		},
	}

	batches := x.getMetricBatches(now, collected)
	batch, ok := batches["http_requests_total"]
	if !ok {
		t.Fatal("expected batch for http_requests_total, got none")
	}
	if len(batch.series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(batch.series))
	}
	got := batch.series[0]
	if got.Sum != 42 {
		t.Errorf("Sum: got %v, want 42", got.Sum)
	}
	// The last dim is always "service".
	last := got.DimValues[len(got.DimValues)-1]
	if last != "svc-a" {
		t.Errorf("service dim: got %q, want %q", last, "svc-a")
	}
}

func TestGetMetricBatches_MultipleServices(t *testing.T) {
	now := time.Now()
	svcs := []string{"svc-a", "svc-b"}
	meta := &metadata.ContainerMetadata{}

	x := newTestExporter(svcs, meta)
	collected := []metrics.CollectedMetric{
		{
			// svcNum=0 means iterate all services.
			Info:  metricInfo{"active_conns", metrics.GaugeType, 0},
			Val:   []float64{10, 20},
			Valid: validBools(2),
		},
	}

	batches := x.getMetricBatches(now, collected)
	batch, ok := batches["active_conns"]
	if !ok {
		t.Fatal("expected batch for active_conns")
	}
	if len(batch.series) != 2 {
		t.Fatalf("expected 2 series (one per service), got %d", len(batch.series))
	}

	// Verify each service has its data.
	svcValues := map[string]float64{}
	for _, s := range batch.series {
		svc := s.DimValues[len(s.DimValues)-1]
		svcValues[svc] = s.Sum
	}
	if svcValues["svc-a"] != 10 {
		t.Errorf("svc-a: got %v, want 10", svcValues["svc-a"])
	}
	if svcValues["svc-b"] != 20 {
		t.Errorf("svc-b: got %v, want 20", svcValues["svc-b"])
	}
}

func TestGetMetricBatches_Labels(t *testing.T) {
	now := time.Now()
	svcs := []string{"svc-a"}
	meta := &metadata.ContainerMetadata{}

	x := newTestExporter(svcs, meta)
	collected := []metrics.CollectedMetric{
		{
			Info:   metricInfo{"cache_hits", metrics.CounterType, 1},
			Labels: []metrics.KeyValue{{Key: "cache_type", Value: "redis"}},
			Val:    []float64{5},
			Valid:  validBools(1),
		},
	}

	batches := x.getMetricBatches(now, collected)
	batch, ok := batches["cache_hits"]
	if !ok {
		t.Fatal("expected batch for cache_hits")
	}
	if len(batch.series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(batch.series))
	}

	// Dim names must contain the label key and "service".
	foundLabel, foundService := false, false
	for _, name := range batch.dimNames {
		if name == "cache_type" {
			foundLabel = true
		}
		if name == "service" {
			foundService = true
		}
	}
	if !foundLabel {
		t.Errorf("dim names %v missing cache_type", batch.dimNames)
	}
	if !foundService {
		t.Errorf("dim names %v missing service", batch.dimNames)
	}
}

func TestGetMetricBatches_Empty(t *testing.T) {
	now := time.Now()
	x := newTestExporter([]string{"svc"}, &metadata.ContainerMetadata{})

	batches := x.getMetricBatches(now, nil)
	if len(batches) != 0 {
		t.Errorf("expected empty batches for nil input, got %d entries", len(batches))
	}

	batches2 := x.getMetricBatches(now, []metrics.CollectedMetric{})
	if len(batches2) != 0 {
		t.Errorf("expected empty batches for empty slice, got %d entries", len(batches2))
	}
}

func TestGetMetricBatches_InvalidMetricSkipped(t *testing.T) {
	now := time.Now()
	svcs := []string{"svc-a"}
	x := newTestExporter(svcs, &metadata.ContainerMetadata{})

	// Valid[0] is false → the metric should be skipped.
	invalid := make([]atomic.Bool, 1)
	invalid[0].Store(false)

	collected := []metrics.CollectedMetric{
		{
			Info:  metricInfo{"skipped_metric", metrics.CounterType, 1},
			Val:   []int64{99},
			Valid: invalid,
		},
	}

	batches := x.getMetricBatches(now, collected)
	if len(batches) != 0 {
		t.Errorf("expected no batches for invalid metric, got %d", len(batches))
	}
}

// ---- getSysBatches tests --------------------------------------------------------------

func TestGetSysBatches(t *testing.T) {
	x := newTestExporter([]string{"svc"}, &metadata.ContainerMetadata{
		ServiceID:  "rg",
		InstanceID: "i1",
	})

	batches := x.getSysBatches(time.Now())

	if _, ok := batches[system.MetricNameHeapObjectsBytes]; !ok {
		t.Errorf("getSysBatches missing %s", system.MetricNameHeapObjectsBytes)
	}
	if _, ok := batches[system.MetricNameGoroutines]; !ok {
		t.Errorf("getSysBatches missing %s", system.MetricNameGoroutines)
	}

	for name, batch := range batches {
		if len(batch.series) != 1 {
			t.Errorf("%s: expected 1 series, got %d", name, len(batch.series))
		}
	}
}

// ---- payload serialization tests -----------------------------------------------------

func TestPayloadSerialization(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	namespace := "Encore/Metrics"
	metricName := "http_requests_total"

	payload := azureCustomMetricPayload{
		Time: now.Format(time.RFC3339),
		Data: azureCustomMetricData{
			BaseData: azureCustomMetricBaseData{
				Metric:    metricName,
				Namespace: namespace,
				DimNames:  []string{"service", "region"},
				Series: []azureCustomMetricSeries{
					{
						DimValues: []string{"svc-a", "eastus"},
						Sum:       42,
						Count:     1,
						Min:       42,
						Max:       42,
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Verify top-level shape.
	if _, ok := got["time"]; !ok {
		t.Error("payload missing 'time' field")
	}
	dataObj, ok := got["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload 'data' field is not an object; got %T", got["data"])
	}
	baseData, ok := dataObj["baseData"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.baseData is not an object; got %T", dataObj["baseData"])
	}

	if baseData["metric"] != metricName {
		t.Errorf("metric: got %v, want %v", baseData["metric"], metricName)
	}
	if baseData["namespace"] != namespace {
		t.Errorf("namespace: got %v, want %v", baseData["namespace"], namespace)
	}

	series, ok := baseData["series"].([]interface{})
	if !ok || len(series) == 0 {
		t.Fatalf("series is missing or empty: %v", baseData["series"])
	}
	s, ok := series[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first series item is not an object; got %T", series[0])
	}

	for _, required := range []string{"sum", "count", "min", "max"} {
		if _, ok := s[required]; !ok {
			t.Errorf("series item missing %q field; got keys: %v", required, keys(s))
		}
	}
}

func keys(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
