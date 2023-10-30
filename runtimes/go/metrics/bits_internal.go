package metrics

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"
)

func getAtomicAdder[V Value]() func(addr *V, delta V) {
	var typ V
	switch any(typ).(type) {
	case int64:
		return func(addr *V, delta V) {
			a := (*int64)(unsafe.Pointer(addr))
			atomic.AddInt64(a, int64(delta))
		}
	case uint64:
		return func(addr *V, delta V) {
			a := (*uint64)(unsafe.Pointer(addr))
			atomic.AddUint64(a, uint64(delta))
		}
	case float64:
		return func(addr *V, delta V) {
			a := (*float64)(unsafe.Pointer(addr))
			atomicAddFloat64(a, float64(delta))
		}
	default:
		panic(fmt.Sprintf("unhandled value type %T", typ))
	}
}

func getAtomicSetter[V Value]() func(addr *V, new V) {
	var typ V
	switch any(typ).(type) {
	case int64:
		return func(addr *V, new V) {
			a := (*int64)(unsafe.Pointer(addr))
			atomic.StoreInt64(a, int64(new))
		}
	case uint64:
		return func(addr *V, new V) {
			a := (*uint64)(unsafe.Pointer(addr))
			atomic.StoreUint64(a, uint64(new))
		}
	case float64:
		return func(addr *V, new V) {
			a := (*float64)(unsafe.Pointer(addr))
			atomicStoreFloat64(a, float64(new))
		}
	default:
		panic(fmt.Sprintf("unhandled value type %T", typ))
	}
}

func getAtomicIncrementer[V Value](adder func(addr *V, delta V)) func(addr *V) {
	return func(addr *V) {
		adder(addr, 1)
	}
}

func atomicAddFloat64(addr *float64, delta float64) float64 {
	for {
		oldBits := atomic.LoadUint64((*uint64)(unsafe.Pointer(addr)))
		newFloat := math.Float64frombits(oldBits) + delta
		newBits := math.Float64bits(newFloat)
		if atomic.CompareAndSwapUint64((*uint64)(unsafe.Pointer(addr)), oldBits, newBits) {
			return newFloat
		}
	}
}

func atomicStoreFloat64(addr *float64, value float64) {
	atomic.StoreUint64((*uint64)(unsafe.Pointer(addr)), math.Float64bits(value))
}

func atomicLoadFloat64(addr *float64) float64 {
	return math.Float64frombits(atomic.LoadUint64((*uint64)(unsafe.Pointer(addr))))
}

type initGate struct {
	state uint32
	mu    sync.Mutex
}

func (g *initGate) Start() {
	if !atomic.CompareAndSwapUint32(&g.state, 0, 1) {
		panic("initGate: already started")
	}
	g.mu.Lock()
}

func (g *initGate) Done() {
	if !atomic.CompareAndSwapUint32(&g.state, 1, 2) {
		panic("initGate: not in state initing")
	}
	g.mu.Unlock()
}

func (g *initGate) Wait() {
	for {
		switch atomic.LoadUint32(&g.state) {
		case 0:
			continue // not started yet
		case 1:
			// It's running, block on the mutex before returning.
			g.mu.Lock()
			g.mu.Unlock()
			return
		case 2:
			return // done
		}
	}
}
