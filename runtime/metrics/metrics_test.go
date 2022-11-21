package metrics

import (
	"reflect"
	"strconv"
	"sync"
	"testing"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/model"
	"encore.dev/appruntime/reqtrack"
)

func TestCounter(t *testing.T) {
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	m := newMetricInfo[int64](mgr, "foo", CounterType, 1)
	c := newCounterInternal(m)

	ts, loaded := m.getTS(nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.info.Name(), "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, ts.value[0], 0)
	c.Increment()
	c.Add(2)
	eq(t, ts.value[0], 3)

	c2 := newCounterInternal(m)
	ts2, loaded2 := c2.getTS(nil)
	eq(t, loaded2, true)
	eq(t, ts2, ts)

	c2.Increment()
	eq(t, ts.value[0], 4)
	eq(t, countryRegistry(&mgr.registry), 1)
}

func TestCounter_MultipleServices(t *testing.T) {
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 2)
	m := newMetricInfo[int64](mgr, "foo", CounterType, 0)
	c := newCounterInternal(m)

	ts, loaded := m.getTS(nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.info.Name(), "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, len(ts.value), 2)
	eq(t, ts.value[0], 0)
	eq(t, ts.value[1], 0)

	// Without a service running these should be no-ops.
	c.Increment()
	c.Add(2)
	eq(t, ts.value[0], 0)
	eq(t, ts.value[1], 0)

	// Inside a request they should work.
	{
		rt.BeginRequest(&model.Request{SvcNum: 1})
		c.Increment()
		c.Add(2)
		eq(t, ts.value[0], 3)
		eq(t, ts.value[1], 0)
		rt.FinishRequest()
	}

	// Without a service running these should be no-ops again.
	c.Increment()
	c.Add(2)
	eq(t, ts.value[0], 3)
	eq(t, ts.value[1], 0)

	// Inside a request they should work.
	{
		rt.BeginRequest(&model.Request{SvcNum: 2})
		c.Increment()
		eq(t, ts.value[0], 3)
		eq(t, ts.value[1], 1)
		rt.FinishRequest()
	}

	// Without a service running these should be no-ops again.
	c.Increment()
	c.Add(2)
	eq(t, ts.value[0], 3)
	eq(t, ts.value[1], 1)
}

func TestGauge(t *testing.T) {
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	m := newMetricInfo[float64](mgr, "foo", GaugeType, 1)
	c := newGauge(m)

	ts, loaded := m.getTS(nil)
	eq(t, loaded, true)
	eq(t, ts.init.state, 2)
	eq(t, ts.info.Name(), "foo")
	if ts.labels != nil {
		t.Fatalf("got labels %+v, want nil", ts.labels)
	}

	eq(t, ts.value[0], 0)
	c.Set(1.5)
	eq(t, ts.value[0], 1.5)

	c2 := newGauge(m)
	ts2, loaded2 := c2.getTS(nil)
	eq(t, loaded2, true)
	eq(t, ts2, ts)

	c2.Set(2)
	eq(t, ts.value[0], 2)

	eq(t, countryRegistry(&mgr.registry), 1)
}

func TestCounterGroup(t *testing.T) {
	type myLabels struct {
		key string
	}

	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	c := newCounterGroup[myLabels, int64](mgr, "foo", CounterConfig{
		EncoreInternal_SvcNum: 1,
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
	eq(t, ts.info.Name(), "foo")
	if !reflect.DeepEqual(ts.labels, []KeyValue{{Key: "Key", Value: "foo"}}) {
		t.Fatalf("got labels %+v, want [{Key foo}]", ts.labels)
	}

	eq(t, ts.value[0], 3)

	ts2 := c.get(myLabels{key: "foo"})
	eq(t, ts2, ts)
	eq(t, countryRegistry(&mgr.registry), 2)

}

func TestGaugeGroup(t *testing.T) {
	type myLabels struct {
		key string
	}
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	c := newGaugeGroup[myLabels, float64](mgr, "foo", GaugeConfig{
		EncoreInternal_SvcNum: 1,
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
	eq(t, ts.info.Name(), "foo")
	if !reflect.DeepEqual(ts.labels, []KeyValue{{Key: "Key", Value: "foo"}}) {
		t.Fatalf("got labels %+v, want [{Key foo}]", ts.labels)
	}

	eq(t, ts.value[0], 2.5)

	ts2 := c.get(myLabels{key: "foo"})
	eq(t, ts2, ts)
	eq(t, countryRegistry(&mgr.registry), 2)
}

func BenchmarkCounter_Inc(b *testing.B) {
	b.ReportAllocs()
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	m := newMetricInfo[int64](mgr, "foo", CounterType, 1)
	c := newCounterInternal(m)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Increment()
	}
	eq(b, c.ts.value[0], int64(b.N))
}

func BenchmarkCounter_NewLabel(b *testing.B) {
	type myLabels struct {
		key string
	}
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	c := newCounterGroup[myLabels, int64](mgr, "foo", CounterConfig{
		EncoreInternal_SvcNum: 1,
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
	rt := reqtrack.New(zerolog.Logger{}, nil, nil)
	mgr := NewRegistry(rt, 1)
	c := newCounterGroup[myLabels, int64](mgr, "foo", CounterConfig{
		EncoreInternal_SvcNum: 1,
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
