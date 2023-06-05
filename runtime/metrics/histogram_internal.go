//go:build encore_app

package metrics

import (
	"math"

	"encore.dev/appruntime/shared/nativehist"
)

// HistogramConfig configures a histogram.
// It's currently a placeholder as there is not yet any additional configuration.
type HistogramConfig struct {
	//publicapigen:drop
	EncoreInternal_LabelMapper any // func(L) []KeyValue

	//publicapigen:drop
	EncoreInternal_SvcNum uint16
}

// NewHistogram creates a new histogram metric, without any labels.
// Use NewHistogramGroup for histograms with labels.
func NewHistogram[V Value](name string, cfg HistogramConfig) *Histogram[V] {
	return newHistogramInternal[V](newMetricInfo[V](Singleton, name, HistogramType, cfg.EncoreInternal_SvcNum))
}

func newHistogramInternal[V Value](m *metricInfo[V]) *Histogram[V] {
	ts, setup := getTS[*nativehist.Histogram](m.reg, m.name, nil, m)

	// Initialize the values if they haven't yet been set up.
	if !setup {
		n := m.reg.numSvcs
		if m.svcNum > 0 {
			n = 1
		}
		ts.value = make([]*nativehist.Histogram, n)
		for i := range ts.value {
			ts.value[i] = nativehist.New(bucketFactor)
		}
	}

	return &Histogram[V]{
		metricInfo: m,
		ts:         ts,
		toFloat:    makeToFloat[V](),
	}
}

type Histogram[V Value] struct {
	*metricInfo[V]
	ts      *timeseries[*nativehist.Histogram]
	toFloat func(V) float64
}

func (h *Histogram[V]) Observe(val V) {
	f := h.toFloat(val)
	if math.IsNaN(f) {
		return
	}
	if idx, ok := h.svcIdx(); ok {
		h.ts.value[idx].Observe(f)
	}
}

// NewHistogramGroup creates a new histogram group with a set of labels,
// where each unique combination of labels becomes its own histogram.
//
// The Labels type must be a named struct, where each field corresponds to
// a single label. Each field must be of type string.
func NewHistogramGroup[L Labels, V Value](name string, cfg HistogramConfig) *HistogramGroup[L, V] {
	return newHistogramGroup[L, V](Singleton, name, cfg)
}

func newHistogramGroup[L Labels, V Value](mgr *Registry, name string, cfg HistogramConfig) *HistogramGroup[L, V] {
	labelMapper := cfg.EncoreInternal_LabelMapper.(func(L) []KeyValue)
	m := newMetricInfo[V](mgr, name, HistogramType, cfg.EncoreInternal_SvcNum)
	return &HistogramGroup[L, V]{
		metricInfo:  m,
		labelMapper: labelMapper,
		toFloat:     makeToFloat[V](),
	}
}

type HistogramGroup[L Labels, V Value] struct {
	*metricInfo[V]
	labelMapper func(L) []KeyValue
	toFloat     func(V) float64
}

func (c *HistogramGroup[L, V]) With(labels L) *Histogram[V] {
	ts := c.get(labels)
	return &Histogram[V]{
		metricInfo: c.metricInfo,
		ts:         ts,
		toFloat:    c.toFloat,
	}
}

func (c *HistogramGroup[L, V]) get(labels L) *timeseries[*nativehist.Histogram] {
	ts, setup := getTS[*nativehist.Histogram](c.reg, c.name, labels, c.metricInfo)

	if !setup {
		n := c.reg.numSvcs
		if c.svcNum > 0 {
			n = 1
		}
		ts.value = make([]*nativehist.Histogram, n)
		for i := range ts.value {
			ts.value[i] = nativehist.New(bucketFactor)
		}
	}

	return ts
}

func makeToFloat[V Value]() func(V) float64 {
	var zero V
	switch any(zero).(type) {
	case int64:
		return func(val V) float64 { return float64(val) }
	case float64:
		return func(val V) float64 { return float64(val) }
	default:
		panic("invalid unit")
	}
}

const bucketFactor = 1.1
