package metrics

import (
	"fmt"
)

type Labels interface {
	comparable
}

type Value interface {
	int64 | float64
}

// CounterConfig configures a counter.
// It's currently a placeholder as there is not yet any additional configuration.
type CounterConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) []KeyValue
}

// NewCounter creates a new counter metric, without any labels.
// Use NewCounterL for metrics with labels.
func NewCounter[V Value](name string, cfg CounterConfig) *Counter[V] {
	return newCounter[V](Singleton, name, cfg)
}

func newCounter[V Value](mgr *Registry, name string, cfg CounterConfig) *Counter[V] {
	ts, setup := getTS[V](mgr, CounterType, name, nil)
	if !setup {
		ts.setupLabels(nil)
	}
	return &Counter[V]{ts: ts, add: getAtomicAdder[V](&ts.value)}
}

type Counter[V Value] struct {
	ts  *timeseries[V]
	add func(delta V)
}

// Increment increments the counter by 1.
func (c *Counter[V]) Increment() {
	c.add(1)
}

// Add adds an arbitrary, non-negative value to the counter.
// It panics if the delta is < 0.
func (c *Counter[V]) Add(delta V) {
	if delta < 0 {
		panic(fmt.Sprintf("metrics: cannot add negative value %v to counter", delta))
	}
	c.add(delta)
}

// NewCounterGroup creates a new counter group with a set of labels,
// where each unique combination of labels becomes its own counter.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewCounterGroup[L Labels, V Value](name string, cfg CounterConfig) *CounterGroup[L, V] {
	return newCounterGroup[L, V](Singleton, name, cfg)
}

func newCounterGroup[L Labels, V Value](mgr *Registry, name string, cfg CounterConfig) *CounterGroup[L, V] {
	labelMapper := cfg.EncoreInternal_LabelMapper.(func(L) []KeyValue)
	return &CounterGroup[L, V]{mgr: mgr, name: name, labelMapper: labelMapper}
}

type CounterGroup[L Labels, V Value] struct {
	mgr         *Registry
	name        string
	labelMapper func(L) []KeyValue
}

func (c *CounterGroup[L, V]) With(labels L) *Counter[V] {
	ts := c.get(labels)
	return &Counter[V]{
		ts:  ts,
		add: getAtomicAdder[V](&ts.value),
	}
}

func (c *CounterGroup[L, V]) get(labels L) *timeseries[V] {
	ts, setup := getTS[V](c.mgr, CounterType, c.name, labels)
	if !setup {
		ts.setupLabels(c.labelMapper(labels))
	}
	return ts
}

// GaugeConfig configures a gauge.
// It's currently a placeholder as there is not yet any additional configuration.
type GaugeConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) any) []KeyValue
}

// NewGauge creates a new counter metric, without any labels.
// Use NewGaugeGroup for metrics with labels.
func NewGauge[V Value](name string, cfg GaugeConfig) *Gauge[V] {
	return newGauge[V](Singleton, name, cfg)
}

func newGauge[V Value](mgr *Registry, name string, cfg GaugeConfig) *Gauge[V] {
	ts, setup := getTS[V](mgr, GaugeType, name, nil)
	if !setup {
		ts.setupLabels(nil)
	}
	return &Gauge[V]{ts: ts, set: getAtomicSetter[V](&ts.value)}
}

type Gauge[V Value] struct {
	ts *timeseries[V]

	set func(val V)
}

func (g *Gauge[V]) Set(val V) {
	g.set(val)
}

// NewGaugeGroup creates a new gauge group with a set of labels,
// where each unique combination of labels becomes its own gauge.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewGaugeGroup[L Labels, V Value](name string, cfg GaugeConfig) *GaugeGroup[L, V] {
	return newGaugeGroup[L, V](Singleton, name, cfg)
}

func newGaugeGroup[L Labels, V Value](mgr *Registry, name string, cfg GaugeConfig) *GaugeGroup[L, V] {
	labelMapper := cfg.EncoreInternal_LabelMapper.(func(L) []KeyValue)
	return &GaugeGroup[L, V]{mgr: mgr, name: name, labelMapper: labelMapper}
}

type GaugeGroup[L Labels, V Value] struct {
	mgr         *Registry
	name        string
	labelMapper func(L) []KeyValue
}

func (g *GaugeGroup[L, V]) With(labels L) *Gauge[V] {
	ts := g.get(labels)
	return &Gauge[V]{ts: ts, set: getAtomicSetter[V](&ts.value)}
}

func (c *GaugeGroup[L, V]) get(labels L) *timeseries[V] {
	ts, setup := getTS[V](c.mgr, GaugeType, c.name, labels)
	if !setup {
		ts.setupLabels(c.labelMapper(labels))
	}
	return ts
}
