//go:build encore_app

package metrics

// NewCounter creates a new counter metric, without any labels.
// Use NewCounterGroup for metrics with labels.
func NewCounter[V Value](name string, cfg CounterConfig) *Counter[V] {
	return newCounterInternal[V](newMetricInfo[V](Singleton, name, CounterType, cfg.EncoreInternal_SvcNum))
}

// NewCounterGroup creates a new counter group with a set of labels,
// where each unique combination of labels becomes its own counter.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewCounterGroup[L Labels, V Value](name string, cfg CounterConfig) *CounterGroup[L, V] {
	return newCounterGroup[L, V](Singleton, name, cfg)
}

// NewGauge creates a new counter metric, without any labels.
// Use NewGaugeGroup for metrics with labels.
func NewGauge[V Value](name string, cfg GaugeConfig) *Gauge[V] {
	return newGauge[V](newMetricInfo[V](Singleton, name, GaugeType, cfg.EncoreInternal_SvcNum))
}

// NewGaugeGroup creates a new gauge group with a set of labels,
// where each unique combination of labels becomes its own gauge.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewGaugeGroup[L Labels, V Value](name string, cfg GaugeConfig) *GaugeGroup[L, V] {
	return newGaugeGroup[L, V](Singleton, name, cfg)
}
