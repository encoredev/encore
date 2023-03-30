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

func doSync(ctx context.Context, cat, name string) func() {
	gid := goroutineID()
	tr := fromCtx(ctx)
	tr.Emit(beginSync, name, cat, nil, gid, 0)
	return func() {
		tr.Emit(endSync, name, cat, nil, gid, 0)
	}
}

var asyncID int64

func doAsync(ctx context.Context, cat, name string) func() {
	id := atomic.AddInt64(&asyncID, 1)
	gid := goroutineID()
	tr := fromCtx(ctx)
	tr.Emit(beginAsync, name, cat, nil, gid, id)
	return func() {
		tr.Emit(endAsync, name, cat, nil, gid, id)
	}
}
