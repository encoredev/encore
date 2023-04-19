package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"

	"encore.dev/appruntime/shared/nativehist"
	"encore.dev/appruntime/shared/reqtrack"
)

type Registry struct {
	rt       *reqtrack.RequestTracker
	numSvcs  uint16
	tsid     uint64
	registry sync.Map // map[registryKey]*timeseries
}

func NewRegistry(rt *reqtrack.RequestTracker, numServicesInBinary int) *Registry {
	return &Registry{rt: rt, numSvcs: uint16(numServicesInBinary)}
}

func (r *Registry) Collect() []CollectedMetric {
	metrics := make([]CollectedMetric, 0, 128)
	r.registry.Range(func(key, value any) bool {
		switch val := value.(type) {
		case *timeseries[int64]:
			metrics = append(metrics, CollectedMetric{
				Info:         val.info,
				TimeSeriesID: val.id,
				Labels:       val.labels,
				Val:          val.value,
				Valid:        val.valid,
			})
		case *timeseries[uint64]:
			metrics = append(metrics, CollectedMetric{
				Info:         val.info,
				TimeSeriesID: val.id,
				Labels:       val.labels,
				Val:          val.value,
				Valid:        val.valid,
			})
		case *timeseries[float64]:
			metrics = append(metrics, CollectedMetric{
				Info:         val.info,
				TimeSeriesID: val.id,
				Labels:       val.labels,
				Val:          val.value,
				Valid:        val.valid,
			})
		case *timeseries[*nativehist.Histogram]:
			metrics = append(metrics, CollectedMetric{
				Info:         val.info,
				TimeSeriesID: val.id,
				Labels:       val.labels,
				Val:          val.value,
				Valid:        val.valid,
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
	HistogramType
)

type MetricInfo interface {
	Name() string
	Type() MetricType
	SvcNum() uint16
}

type CollectedMetric struct {
	Info         MetricInfo
	TimeSeriesID uint64
	Labels       []KeyValue
	Val          any // []T where T is any of Value
	Valid        []atomic.Bool
}

type registryKey struct {
	metricName string
	labels     any // guaranteed to be comparable
}

type timeseries[T any] struct {
	info   MetricInfo
	id     uint64
	init   initGate
	labels []KeyValue
	value  []T
	valid  []atomic.Bool
}

func (ts *timeseries[V]) setup(labels []KeyValue) {
	ts.init.Start()
	defer ts.init.Done()
	ts.labels = labels
}

type KeyValue struct {
	Key   string
	Value string
}

func getTS[T any](r *Registry, name string, labels any, info MetricInfo) (ts *timeseries[T], loaded bool) {
	key := registryKey{metricName: name, labels: labels}
	if val, ok := r.registry.Load(key); ok {
		return val.(*timeseries[T]), true
	}
	val, loaded := r.registry.LoadOrStore(key, &timeseries[T]{
		info: info,
		id:   atomic.AddUint64(&r.tsid, 1),
	})
	return val.(*timeseries[T]), loaded
}
