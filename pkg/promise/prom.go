package promise

import (
	"context"
	"sync"
)

type Value[T any] struct {
	val  T
	err  error
	done chan struct{}

	onResolve eventList[T]
	onReject  eventList[error]
}

func (v *Value[T]) Get(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case <-v.done:
		return v.val, v.err
	}
}

func (v *Value[T]) OnResolve(fn func(T)) {
	v.onResolve.Add(fn)
}

func (v *Value[T]) OnReject(fn func(error)) {
	v.onReject.Add(fn)
}

func New[T any](fn func() (T, error)) *Value[T] {
	val := &Value[T]{
		done: make(chan struct{}),
	}
	go func() {
		val.val, val.err = fn()
		close(val.done)

		if val.err == nil {
			val.onResolve.MarkDoneAndProcess(val.val)
		} else {
			val.onReject.MarkDoneAndProcess(val.err)
		}
	}()
	return val
}

func Resolved[T any](val T) *Value[T] {
	done := make(chan struct{})
	close(done)
	v := &Value[T]{
		done: done,
		val:  val,
	}
	v.onResolve.MarkDoneAndProcess(val)
	return v
}

func Rejected[T any](err error) *Value[T] {
	done := make(chan struct{})
	close(done)
	v := &Value[T]{
		done: done,
		err:  err,
	}
	v.onReject.MarkDoneAndProcess(err)
	return v
}

type eventList[V any] struct {
	mu    sync.Mutex
	done  bool
	val   V
	funcs []func(V)
}

func (g *eventList[V]) Add(fn func(V)) {
	g.mu.Lock()
	if g.done {
		g.mu.Unlock()
		fn(g.val)
	} else {
		g.funcs = append(g.funcs, fn)
		g.mu.Unlock()
	}
}

func (g *eventList[V]) MarkDoneAndProcess(val V) {
	g.mu.Lock()
	g.done = true
	g.val = val
	g.mu.Unlock()

	for _, fn := range g.funcs {
		fn(val)
	}
}

func Wait2[A, B any](ctx context.Context, a *Value[A], b *Value[B]) (A, B, error) {
	aVal, err1 := a.Get(ctx)
	bVal, err2 := b.Get(ctx)

	err := err1
	if err == nil {
		err = err2
	}
	return aVal, bVal, err
}

func Wait3[A, B, C any](ctx context.Context, a *Value[A], b *Value[B], c *Value[C]) (A, B, C, error) {
	aVal, err1 := a.Get(ctx)
	bVal, err2 := b.Get(ctx)
	cVal, err3 := c.Get(ctx)

	err := err1
	if err == nil {
		err = err2
	}
	if err == nil {
		err = err3
	}
	return aVal, bVal, cVal, err
}
