package metrics

type Labels interface {
	comparable
}

// CounterConfig configures a counter.
// It's currently a placeholder as there is not yet any additional configuration.
type CounterConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) []keyValue
}

// NewCounter creates a new counter metric, without any labels.
// Use NewCounterL for metrics with labels.
func NewCounter(name string, cfg CounterConfig) *Counter {
	return newCounter(Singleton, name, cfg)
}

func newCounter(mgr *Manager, name string, cfg CounterConfig) *Counter {
	ts, loaded := getTS[counterData](mgr, name, nil)
	if !loaded {
		ts.setup(name, nil)
	}
	return &Counter{ts: ts}
}

type Counter struct {
	ts *timeseries[counterData]
}

// Increment increments the counter by 1.
func (c *Counter) Increment() {
	c.ts.data.Increment()
}

// Add adds an arbitrary, non-negative value to the counter.
// It panics if the delta is < 0.
func (c *Counter) Add(delta float64) {
	c.ts.data.Add(delta)
}

// NewCounterL creates a new counter metric with a set of labels.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewCounterL[L Labels](name string, cfg CounterConfig) *CounterL[L] {
	return newCounterL[L](Singleton, name, cfg)
}

func newCounterL[L Labels](mgr *Manager, name string, cfg CounterConfig) *CounterL[L] {
	labelMapper := cfg.EncoreInternal_LabelMapper.(func(L) []keyValue)
	return &CounterL[L]{mgr: mgr, name: name, labelMapper: labelMapper}
}

type CounterL[L Labels] struct {
	mgr         *Manager
	name        string
	labelMapper func(L) []keyValue
}

// Increment increments the counter by 1 for the time series with the given label.
func (c *CounterL[L]) Increment(labels L) {
	c.get(labels).data.Increment()
}

// Add adds an arbitrary, non-negative value to the counter for the time series with the given label.
// It panics if the delta is < 0.
func (c *CounterL[L]) Add(labels L, delta float64) {
	c.get(labels).data.Add(delta)
}

func (c *CounterL[L]) WithLabels(labels L) *Counter {
	return &Counter{
		ts: c.get(labels),
	}
}

func (c *CounterL[L]) get(labels L) *timeseries[counterData] {
	ts, loaded := getTS[counterData](c.mgr, c.name, labels)
	if !loaded {
		ts.setup(c.name, c.labelMapper(labels))
	}
	return ts
}

// GaugeConfig configures a gauge.
// It's currently a placeholder as there is not yet any additional configuration.
type GaugeConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) any) []keyValue
}

// NewGauge creates a new counter metric, without any labels.
// Use NewGaugeL for metrics with labels.
func NewGauge(name string, cfg GaugeConfig) *Gauge {
	return newGauge(Singleton, name, cfg)
}

func newGauge(mgr *Manager, name string, cfg GaugeConfig) *Gauge {
	ts, loaded := getTS[gaugeData](mgr, name, nil)
	if !loaded {
		ts.setup(name, nil)
	}
	return &Gauge{ts: ts}
}

type Gauge struct {
	ts *timeseries[gaugeData]
}

func (g *Gauge) Set(val float64) {
	g.ts.data.Set(val)
}

// NewGaugeL creates a new counter metric, without any labels.
func NewGaugeL[L Labels](name string, cfg GaugeConfig) *GaugeL[L] {
	return newGaugeL[L](Singleton, name, cfg)
}

func newGaugeL[L Labels](mgr *Manager, name string, cfg GaugeConfig) *GaugeL[L] {
	labelMapper := cfg.EncoreInternal_LabelMapper.(func(L) []keyValue)
	return &GaugeL[L]{mgr: mgr, name: name, labelMapper: labelMapper}
}

type GaugeL[L Labels] struct {
	mgr         *Manager
	name        string
	labelMapper func(L) []keyValue
}

func (g *GaugeL[L]) Set(labels L, value float64) {
	g.get(labels).data.Set(value)
}

func (g *GaugeL[L]) WithLabels(labels L) *Gauge {
	return &Gauge{ts: g.get(labels)}
}

func (c *GaugeL[L]) get(labels L) *timeseries[gaugeData] {
	ts, loaded := getTS[gaugeData](c.mgr, c.name, labels)
	if !loaded {
		ts.setup(c.name, c.labelMapper(labels))
	}
	return ts
}
