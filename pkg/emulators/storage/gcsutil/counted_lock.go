package gcsutil

import (
	"context"
)

type countedLock struct {
	// a 1 element channel; empty == unlocked, full == locked
	// push an element into the channel to lock, remove the element to unlock
	ch chan struct{}

	// should only be accessed while the outer _map_ lock is held (not this key lock)
	refcount int64
}

func newCountedLock() *countedLock {
	return &countedLock{
		ch:       make(chan struct{}, 1),
		refcount: 0,
	}
}

func (m *countedLock) Lock(ctx context.Context) bool {
	// If the context is already cancelled don't even try to lock.
	if ctx.Err() != nil {
		return false
	}
	select {
	case m.ch <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *countedLock) Unlock() {
	select {
	case <-m.ch:
		return
	default:
		panic("BUG: lock not held")
	}
}

func (m *countedLock) Run(ctx context.Context, f func(ctx context.Context) error) error {
	if !m.Lock(ctx) {
		return ctx.Err()
	}
	defer m.Unlock()
	return f(ctx)
}
