package metrics

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	mgr := NewRegistry()
	c := newCounter[int64](mgr, "foo", CounterConfig{})

	ts, loaded := getTS[int64](mgr, CounterType, "foo", nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, ts.value, 0)
	c.Increment()
	c.Add(2)
	eq(t, ts.value, 3)

	c2 := newCounter[int64](mgr, "foo", CounterConfig{})
	ts2, loaded2 := getTS[int64](mgr, CounterType, "foo", nil)
	eq(t, loaded2, true)
	eq(t, ts2, ts)

	c2.Increment()
	eq(t, ts.value, 4)
	eq(t, countryRegistry(&mgr.registry), 1)
}

func TestGauge(t *testing.T) {
	mgr := NewRegistry()
	c := newGauge[float64](mgr, "foo", GaugeConfig{})

	ts, loaded := getTS[float64](mgr, GaugeType, "foo", nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, ts.value, 0)
	c.Set(1.5)
	eq(t, ts.value, 1.5)

	c2 := newGauge[float64](mgr, "foo", GaugeConfig{})
	ts2, loaded2 := getTS[float64](mgr, GaugeType, "foo", nil)
	eq(t, loaded2, true)
	eq(t, ts2, ts)

	c2.Set(2)
	eq(t, ts.value, 2)

	eq(t, countryRegistry(&mgr.registry), 1)
}

func TestCounterGroup(t *testing.T) {
	type myLabels struct {
		key string
	}
	mgr := NewRegistry()
	c := newCounterGroup[myLabels, int64](mgr, "foo", CounterConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []KeyValue {
			return []KeyValue{{Key: "Key", Value: labels.key}}
		},
	})

	// GaugeGroup loads time series on-demand.
	eq(t, countryRegistry(&mgr.registry), 0)
	c.With(myLabels{key: "foo"}).Increment()
	eq(t, countryRegistry(&mgr.registry), 1)
	c.With(myLabels{key: "foo"}).Add(2)
	eq(t, countryRegistry(&mgr.registry), 1)
	c.With(myLabels{key: "bar"}).Add(5)
	eq(t, countryRegistry(&mgr.registry), 2)

	ts := c.get(myLabels{key: "foo"})
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if !reflect.DeepEqual(ts.labels, []KeyValue{{Key: "Key", Value: "foo"}}) {
		t.Fatalf("got labels %+v, want [{Key foo}]", ts.labels)
	}

	eq(t, ts.value, 3)

	ts2 := c.get(myLabels{key: "foo"})
	eq(t, ts2, ts)
	eq(t, countryRegistry(&mgr.registry), 2)

}

func TestGaugeGroup(t *testing.T) {
	type myLabels struct {
		key string
	}
	mgr := NewRegistry()
	c := newGaugeGroup[myLabels, float64](mgr, "foo", GaugeConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []KeyValue {
			return []KeyValue{{Key: "Key", Value: labels.key}}
		},
	})

	// GaugeGroup loads time series on-demand.
	eq(t, countryRegistry(&mgr.registry), 0)
	c.With(myLabels{key: "foo"}).Set(1.5)
	eq(t, countryRegistry(&mgr.registry), 1)
	c.With(myLabels{key: "foo"}).Set(2.5)
	eq(t, countryRegistry(&mgr.registry), 1)
	c.With(myLabels{key: "bar"}).Set(3.5)
	eq(t, countryRegistry(&mgr.registry), 2)

	ts := c.get(myLabels{key: "foo"})
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if !reflect.DeepEqual(ts.labels, []KeyValue{{Key: "Key", Value: "foo"}}) {
		t.Fatalf("got labels %+v, want [{Key foo}]", ts.labels)
	}

	eq(t, ts.value, 2.5)

	ts2 := c.get(myLabels{key: "foo"})
	eq(t, ts2, ts)
	eq(t, countryRegistry(&mgr.registry), 2)
}

func BenchmarkCounter_Inc(b *testing.B) {
	b.ReportAllocs()
	mgr := NewRegistry()
	c := newCounter[int64](mgr, "foo", CounterConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Increment()
	}
	eq(b, c.ts.value, int64(b.N))
}

func BenchmarkCounter_NewLabel(b *testing.B) {
	type myLabels struct {
		key string
	}
	mgr := NewRegistry()
	c := newCounterGroup[myLabels, int64](mgr, "foo", CounterConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []KeyValue {
			return []KeyValue{{Key: "Key", Value: labels.key}}
		},
	})

	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = strconv.Itoa(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.With(myLabels{key: keys[i]}).Increment()
	}
}

func BenchmarkCounter_NewLabelSometimes(b *testing.B) {
	type myLabels struct {
		key string
	}
	mgr := NewRegistry()
	c := newCounterGroup[myLabels, int64](mgr, "foo", CounterConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []KeyValue {
			return []KeyValue{{Key: "Key", Value: labels.key}}
		},
	})

	denom := 10
	numLabels := b.N / denom

	keys := make([]string, numLabels)
	for i := 0; i < numLabels; i++ {
		keys[i] = strconv.Itoa(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < numLabels; i++ {
		for j := 0; j < denom; j++ {
			c.With(myLabels{key: keys[i]}).Increment()
		}
	}
}

func eq[Val comparable](t testing.TB, got, want Val) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func countryRegistry(reg *sync.Map) int {
	var count int
	reg.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}
