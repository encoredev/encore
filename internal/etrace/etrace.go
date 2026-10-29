package etrace

import (
	"context"
	"sync/atomic"
)

func Sync0(ctx context.Context, cat, name string, fn func(context.Context)) {
	defer doSync(ctx, cat, name)()
	fn(ctx)
}

func Sync1[A any](ctx context.Context, cat, name string, fn func(context.Context) A) A {
	defer doSync(ctx, cat, name)()
	return fn(ctx)
}

func Sync2[A, B any](ctx context.Context, cat, name string, fn func(context.Context) (A, B)) (A, B) {
	defer doSync(ctx, cat, name)()
	return fn(ctx)
}

func Async0(ctx context.Context, cat, name string, fn func(context.Context)) {
	defer doAsync(ctx, cat, name)()
	fn(ctx)
}

func Async1[A any](ctx context.Context, cat, name string, fn func(context.Context) A) A {
	defer doAsync(ctx, cat, name)()
	return fn(ctx)
}

func Async2[A, B any](ctx context.Context, cat, name string, fn func(context.Context) (A, B)) (A, B) {
	defer doAsync(ctx, cat, name)()
	return fn(ctx)
}

func noop() {}

func doSync(ctx context.Context, cat, name string) func() {
	tr := fromCtx(ctx)
	if tr == nil {
		return noop
	}
	gid := goroutineID()
	tr.Emit(beginSync, name, cat, nil, gid, 0)
	return func() {
		tr.Emit(endSync, name, cat, nil, gid, 0)
	}
}

var asyncID int64

func doAsync(ctx context.Context, cat, name string) func() {
	tr := fromCtx(ctx)
	if tr == nil {
		return noop
	}
	id := atomic.AddInt64(&asyncID, 1)
	gid := goroutineID()
	tr.Emit(beginAsync, name, cat, nil, gid, id)
	return func() {
		tr.Emit(endAsync, name, cat, nil, gid, id)
	}
}
