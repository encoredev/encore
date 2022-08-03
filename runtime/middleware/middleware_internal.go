package middleware

import (
	"context"
	"sync"

	encore "encore.dev"
)

func newReqCache(fn func() *encore.Request) *reqCache {
	return &reqCache{load: fn}
}

type reqCache struct {
	loadOnce sync.Once
	load     func() *encore.Request
	req      *encore.Request
}

func (r *reqCache) Get() *encore.Request {
	r.loadOnce.Do(func() { r.req = r.load() })
	return r.req
}

//publicapigen:drop
func NewLazyRequest(ctx context.Context, fn func() *encore.Request) Request {
	return Request{ctx: ctx, cache: newReqCache(fn)}
}
