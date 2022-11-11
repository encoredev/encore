package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var Singleton *Registry // TODO

type Registry struct {
	tsid     uint64
	registry sync.Map // map[registryKey]*timeseries
}

func NewRegistry() *Registry {
	return &Registry{}
}

// TODO this is awful
func (r *Registry) Collect() []CollectedMetric {
	metrics := make([]CollectedMetric, 0, 100) // TODO
	r.registry.Range(func(key, value any) bool {
		k := key.(registryKey)
		switch val := value.(type) {
		case *timeseries[int64]:
			metrics = append(metrics, CollectedMetric{
				MetricName:   k.metricName,
				Type:         val.typ,
				TimeSeriesID: val.id,
				Labels:       val.labels,
				Val:          val.value,
			})
		case *timeseries[float64]:
			metrics = append(metrics, CollectedMetric{
				MetricName:   k.metricName,
				Type:         val.typ,
				TimeSeriesID: val.id,
				Labels:       val.labels,
				Val:          val.value,
			})
		default:
			panic(fmt.Sprintf("unhandled timeseries type %T", val))
		}
		return true
	})
	return metrics
}

type MetricType int

const (
	CounterType MetricType = iota
	GaugeType
)

type CollectedMetric struct {
	MetricName   string
	Type         MetricType
	TimeSeriesID uint64
	Labels       []KeyValue
	Val          any
}

type registryKey struct {
	metricName string
	labels     any // guaranteed to be comparable
}

type timeseries[T any] struct {
	id         uint64
	typ        MetricType
	init       initGate
	metricName string
	labels     []KeyValue
	value      T
}

func (ts *timeseries[V]) setupLabels(labels []KeyValue) {
	ts.init.Start()
	defer ts.init.Done()
	ts.labels = labels
}

type KeyValue struct {
	Key   string
	Value string
}

func getTS[T any](r *Registry, typ MetricType, name string, labels any) (ts *timeseries[T], loaded bool) {
	key := registryKey{metricName: name, labels: labels}
	if val, ok := r.registry.Load(key); ok {
		return val.(*timeseries[T]), true
	}
	val, loaded := r.registry.LoadOrStore(key, &timeseries[T]{
		typ:        typ,
		metricName: name,
		id:         atomic.AddUint64(&r.tsid, 1),
	})
	return val.(*timeseries[T]), loaded
}
