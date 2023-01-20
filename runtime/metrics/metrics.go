package metrics

import (
	"fmt"
	"sync/atomic"
)

type Labels interface {
	comparable
}

// CounterConfig configures a counter.
// It's currently a placeholder as there is not yet any additional configuration.
type CounterConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) []KeyValue

	//publicapigen:drop
	EncoreInternal_SvcNum uint16
}

// NewCounter creates a new counter metric, without any labels.
// Use NewCounterGroup for metrics with labels.
func NewCounter[V Value](name string, cfg CounterConfig) *Counter[V] {
	return newCounterInternal[V](newMetricInfo[V](Singleton, name, CounterType, cfg.EncoreInternal_SvcNum))
}

func newCounterInternal[V Value](m *metricInfo[V]) *Counter[V] {
	ts, setup := m.getTS(nil)
	if !setup {
		ts.setup(nil)
	}
	return &Counter[V]{metricInfo: m, ts: ts}
}

type Counter[V Value] struct {
	*metricInfo[V]
	ts *timeseries[V]
}

// Increment increments the counter by 1.
func (c *Counter[V]) Increment() {
	if idx, ok := c.svcIdx(); ok {
		c.inc(&c.ts.value[idx])
		if !c.ts.valid[idx].Load() {
			c.ts.valid[idx].Store(true)
		}
	}
}

// Add adds an arbitrary, non-negative value to the counter.
// It panics if the delta is < 0.
func (c *Counter[V]) Add(delta V) {
	if delta < 0 {
		panic(fmt.Sprintf("metrics: cannot add negative value %v to counter", delta))
	}
	if idx, ok := c.svcIdx(); ok {
		c.add(&c.ts.value[idx], delta)
		if !c.ts.valid[idx].Load() {
			c.ts.valid[idx].Store(true)
		}
	}
}

// NewCounterGroup creates a new counter group with a set of labels,
// where each unique combination of labels becomes its own counter.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewCounterGroup[L Labels, V Value](name string, cfg CounterConfig) *CounterGroup[L, V] {
	return newCounterGroup[L, V](Singleton, name, cfg)
}

//publicapigen:drop
func NewCounterGroupInternal[L Labels, V Value](reg *Registry, name string, cfg CounterConfig) *CounterGroup[L, V] {
	return newCounterGroup[L, V](reg, name, cfg)
}

func newCounterGroup[L Labels, V Value](mgr *Registry, name string, cfg CounterConfig) *CounterGroup[L, V] {
	labelMapper := cfg.EncoreInternal_LabelMapper.(func(L) []KeyValue)
	m := newMetricInfo[V](mgr, name, CounterType, cfg.EncoreInternal_SvcNum)
	return &CounterGroup[L, V]{metricInfo: m, labelMapper: labelMapper}
}

type CounterGroup[L Labels, V Value] struct {
	*metricInfo[V]
	labelMapper func(L) []KeyValue
}

func (c *CounterGroup[L, V]) With(labels L) *Counter[V] {
	ts := c.get(labels)
	return &Counter[V]{metricInfo: c.metricInfo, ts: ts}
}

func (c *CounterGroup[L, V]) get(labels L) *timeseries[V] {
	ts, setup := c.metricInfo.getTS(labels)
	if !setup {
		ts.setup(c.labelMapper(labels))
	}
	return ts
}

// GaugeConfig configures a gauge.
// It's currently a placeholder as there is not yet any additional configuration.
type GaugeConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) any) []KeyValue

	//publicapigen:drop
	EncoreInternal_SvcNum uint16
}

// NewGauge creates a new counter metric, without any labels.
// Use NewGaugeGroup for metrics with labels.
func NewGauge[V Value](name string, cfg GaugeConfig) *Gauge[V] {
	return newGauge[V](newMetricInfo[V](Singleton, name, GaugeType, cfg.EncoreInternal_SvcNum))
}

func newGauge[V Value](m *metricInfo[V]) *Gauge[V] {
	ts, setup := m.getTS(nil)
	if !setup {
		ts.setup(nil)
	}

	return &Gauge[V]{metricInfo: m, ts: ts}
}

type Gauge[V Value] struct {
	*metricInfo[V]
	ts *timeseries[V]
}

func (g *Gauge[V]) Set(val V) {
	if idx, ok := g.svcIdx(); ok {
		g.set(&g.ts.value[idx], val)
		if !g.ts.valid[idx].Load() {
			g.ts.valid[idx].Store(true)
		}
	}
}

func (g *Gauge[V]) Add(val V) {
	if idx, ok := g.svcIdx(); ok {
		g.add(&g.ts.value[idx], val)
		if !g.ts.valid[idx].Load() {
			g.ts.valid[idx].Store(true)
		}
	}
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
	m := newMetricInfo[V](mgr, name, GaugeType, cfg.EncoreInternal_SvcNum)
	return &GaugeGroup[L, V]{metricInfo: m, labelMapper: labelMapper}
}

type GaugeGroup[L Labels, V Value] struct {
	*metricInfo[V]
	labelMapper func(L) []KeyValue
}

func (g *GaugeGroup[L, V]) With(labels L) *Gauge[V] {
	ts := g.get(labels)
	return &Gauge[V]{metricInfo: g.metricInfo, ts: ts}
}

func (g *GaugeGroup[L, V]) get(labels L) *timeseries[V] {
	ts, setup := g.metricInfo.getTS(labels)
	if !setup {
		ts.setup(g.labelMapper(labels))
	}
	return ts
}

func newMetricInfo[V Value](mgr *Registry, name string, typ MetricType, svcNum uint16) *metricInfo[V] {
	add := getAtomicAdder[V]()
	set := getAtomicSetter[V]()
	inc := getAtomicIncrementer[V](add)

	return &metricInfo[V]{
		reg:    mgr,
		name:   name,
		typ:    typ,
		svcNum: svcNum,

		add: add,
		set: set,
		inc: inc,
	}
}

type metricInfo[V Value] struct {
	reg    *Registry
	name   string
	typ    MetricType
	svcNum uint16

	add func(addr *V, val V)
	set func(addr *V, val V)
	inc func(addr *V)
}

func (m *metricInfo[V]) svcIdx() (idx uint16, ok bool) {
	if m.svcNum > 0 {
		return 0, true
	} else if curr := m.reg.rt.Current(); curr.SvcNum > 0 {
		return curr.SvcNum - 1, true
	}
	return 0, false
}

func (m *metricInfo[V]) getTS(labels any) (ts *timeseries[V], setup bool) {
	ts, setup = getTS[V](m.reg, m.name, labels, m)

	// Initialize the values if they haven't yet been set up.
	if !setup {
		if m.svcNum > 0 {
			ts.value = make([]V, 1)
			ts.valid = make([]atomic.Bool, 1)
			ts.valid[0].Store(true)
		} else {
			n := m.reg.numSvcs
			ts.value = make([]V, n)
			ts.valid = make([]atomic.Bool, n)
		}
	}

	return ts, setup
}

func (m *metricInfo[V]) Name() string     { return m.name }
func (m *metricInfo[V]) Type() MetricType { return m.typ }
func (m *metricInfo[V]) SvcNum() uint16   { return m.svcNum }
