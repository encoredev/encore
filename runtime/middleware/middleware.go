package middleware

import (
	"context"

	encore "encore.dev"
)

type Request struct {
	ctx   context.Context
	cache *reqCache
}

func (r *Request) WithContext(ctx context.Context) Request {
	r2 := *r
	r2.ctx = ctx
	return r2
}

func (r *Request) Context() context.Context { return r.ctx }

func (r *Request) Data() *encore.Request {
	return r.cache.Get()
}

type Next func(Request) Response

type Response struct {
	Payload    any
	Err        error
	HTTPStatus int // if non-zero, gets used as status code
}

type Signature func(req Request, next Next) Response

func NewRequest(ctx context.Context, req *encore.Request) Request {
	return Request{
		ctx:   ctx,
		cache: newReqCache(func() *encore.Request { return req }),
	}
}
