package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var Singleton *Manager // TODO

type Manager struct {
	registry sync.Map // map[registryKey]*timeseries
}

func NewManager() *Manager {
	return &Manager{}
}

type registryKey struct {
	metricName string
	labels     any // guaranteed to be comparable
}

type timeseries[T any] struct {
	init       initGate
	metricName string
	labels     []keyValue
	data       T
}

func (ts *timeseries[T]) setup(metricName string, labels []keyValue) {
	ts.init.Start()
	defer ts.init.Done()
	ts.metricName = metricName
	ts.labels = labels
}

type keyValue struct {
	key   string
	value string
}

type counterData struct {
	intVal   uint64
	floatVal float64
}

func (d *counterData) Increment() {
	atomic.AddUint64(&d.intVal, 1)
}

func (d *counterData) Add(delta float64) {
	if delta < 0 {
		panic(fmt.Sprintf("metrics: cannot add negative value %f to counter", delta))
	}
	atomicAddFloat64(&d.floatVal, delta)
}

type gaugeData struct {
	val float64
}

func (d *gaugeData) Set(val float64) {
	atomicStoreFloat64(&d.val, val)
}

func (d *gaugeData) Get() float64 {
	return atomicLoadFloat64(&d.val)
}

func getTS[T any](mgr *Manager, name string, labels any) (ts *timeseries[T], loaded bool) {
	key := registryKey{metricName: name, labels: labels}
	if val, ok := mgr.registry.Load(key); ok {
		return val.(*timeseries[T]), true
	}
	val, loaded := mgr.registry.LoadOrStore(key, &timeseries[T]{})
	return val.(*timeseries[T]), loaded
}
