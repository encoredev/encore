package metrics

import (
	"math"
	"sync"
	"sync/atomic"
	"unsafe"
)

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
