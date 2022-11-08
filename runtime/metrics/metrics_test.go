package metrics

import (
	"reflect"
	"strconv"
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	mgr := NewManager()
	c := newCounter(mgr, "foo", CounterConfig{})

	ts, loaded := getTS[counterData](mgr, "foo", nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, ts.data.intVal, 0)
	eq(t, ts.data.floatVal, 0)
	c.Increment()
	c.Add(1.5)
	eq(t, ts.data.intVal, 1)
	eq(t, ts.data.floatVal, 1.5)

	c2 := newCounter(mgr, "foo", CounterConfig{})
	ts2, loaded2 := getTS[counterData](mgr, "foo", nil)
	eq(t, loaded2, true)
	eq(t, ts2, ts)

	c2.Increment()
	eq(t, ts.data.intVal, 2)
	eq(t, countryRegistry(&mgr.registry), 1)
}

func TestGauge(t *testing.T) {
	mgr := NewManager()
	c := newGauge(mgr, "foo", GaugeConfig{})

	ts, loaded := getTS[gaugeData](mgr, "foo", nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, ts.data.val, 0)
	c.Set(1.5)
	eq(t, ts.data.val, 1.5)

	c2 := newGauge(mgr, "foo", GaugeConfig{})
	ts2, loaded2 := getTS[gaugeData](mgr, "foo", nil)
	eq(t, loaded2, true)
	eq(t, ts2, ts)

	c2.Set(2)
	eq(t, ts.data.val, 2)

	eq(t, countryRegistry(&mgr.registry), 1)
}

func TestCounterL(t *testing.T) {
	type myLabels struct {
		key string
	}
	mgr := NewManager()
	c := newCounterL[myLabels](mgr, "foo", CounterConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []keyValue {
			return []keyValue{{key: "key", value: labels.key}}
		},
	})

	// GaugeL loads time series on-demand.
	eq(t, countryRegistry(&mgr.registry), 0)
	c.Increment(myLabels{key: "foo"})
	eq(t, countryRegistry(&mgr.registry), 1)
	c.Add(myLabels{key: "foo"}, 2)
	eq(t, countryRegistry(&mgr.registry), 1)
	c.Add(myLabels{key: "bar"}, 5)
	eq(t, countryRegistry(&mgr.registry), 2)

	ts := c.get(myLabels{key: "foo"})
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if !reflect.DeepEqual(ts.labels, []keyValue{{key: "key", value: "foo"}}) {
		t.Fatalf("got labels %+v, want [{key foo}]", ts.labels)
	}

	eq(t, ts.data.intVal, 1)
	eq(t, ts.data.floatVal, 2)

	ts2 := c.get(myLabels{key: "foo"})
	eq(t, ts2, ts)
	eq(t, countryRegistry(&mgr.registry), 2)

}

func TestGaugeL(t *testing.T) {
	type myLabels struct {
		key string
	}
	mgr := NewManager()
	c := newGaugeL[myLabels](mgr, "foo", GaugeConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []keyValue {
			return []keyValue{{key: "key", value: labels.key}}
		},
	})

	// GaugeL loads time series on-demand.
	eq(t, countryRegistry(&mgr.registry), 0)
	c.Set(myLabels{key: "foo"}, 1.5)
	eq(t, countryRegistry(&mgr.registry), 1)
	c.Set(myLabels{key: "foo"}, 2.5)
	eq(t, countryRegistry(&mgr.registry), 1)
	c.Set(myLabels{key: "bar"}, 3.5)
	eq(t, countryRegistry(&mgr.registry), 2)

	ts := c.get(myLabels{key: "foo"})
	eq(t, ts.init.state, 2)
	eq(t, ts.metricName, "foo")
	if !reflect.DeepEqual(ts.labels, []keyValue{{key: "key", value: "foo"}}) {
		t.Fatalf("got labels %+v, want [{key foo}]", ts.labels)
	}

	eq(t, ts.data.val, 2.5)

	ts2 := c.get(myLabels{key: "foo"})
	eq(t, ts2, ts)
	eq(t, countryRegistry(&mgr.registry), 2)
}

func BenchmarkCounter_Inc(b *testing.B) {
	b.ReportAllocs()
	mgr := NewManager()
	c := newCounter(mgr, "foo", CounterConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Increment()
	}
	eq(b, c.ts.data.intVal, uint64(b.N))
}

func BenchmarkCounter_NewLabel(b *testing.B) {
	type myLabels struct {
		key string
	}
	mgr := NewManager()
	c := newCounterL[myLabels](mgr, "foo", CounterConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []keyValue {
			return []keyValue{{key: "key", value: labels.key}}
		},
	})

	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = strconv.Itoa(i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Increment(myLabels{key: keys[i]})
	}
}

func BenchmarkCounter_NewLabelSometimes(b *testing.B) {
	type myLabels struct {
		key string
	}
	mgr := NewManager()
	c := newCounterL[myLabels](mgr, "foo", CounterConfig{
		EncoreInternal_LabelMapper: func(labels myLabels) []keyValue {
			return []keyValue{{key: "key", value: labels.key}}
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
			c.Increment(myLabels{key: keys[i]})
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
