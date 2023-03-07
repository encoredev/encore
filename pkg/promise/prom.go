package promise

import "context"

type Value[T any] struct {
	val  T
	err  error
	done chan struct{}
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

func New[T any](fn func() (T, error)) *Value[T] {
	val := &Value[T]{
		done: make(chan struct{}),
	}
	go func() {
		defer close(val.done)
		val.val, val.err = fn()
	}()
	return val
}
