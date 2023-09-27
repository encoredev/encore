// Package middleware provides middleware functionality for defining
// generic processing across multiple API endpoints, typically for
// cross-cutting concerns like validation, caching, or error monitoring.
//
// For documentation on how to use middleware in Encore see https://encore.dev/docs/develop/middleware.
package middleware

import (
	"context"

	encore "encore.dev"
)

// Signature is the signature a middleware should have.
// The implementation must return a Response which then
// becomes the API's response.
//
// Middleware processing forms a chain, where the first
// defined middleware is called first. The provided next
// function is used to invoke the next middleware in the
// chain, or the actual API handler if it's the last middleware.
//
// The implementation should call next at most once.
// If the middleware encounters an error it is permissible to
// not call next and directly return a Response with Err set
// to a non-nil value.
//
// The type exists solely for documentation purposes;
// it is otherwise not used.
type Signature func(req Request, next Next) Response

// Next is the function to invoke the next middleware in the chain,
// or the actual API handler if the middleware is the last one.
type Next func(Request) Response

// A Request provides information about the incoming request that
// the middleware is processing.
type Request struct {
	ctx   context.Context
	cache *reqCache
}

// WithContext returns a new Request with the context set to ctx.
func (r *Request) WithContext(ctx context.Context) Request {
	r2 := *r
	r2.ctx = ctx
	return r2
}

// Context reports the request's context.
func (r *Request) Context() context.Context {
	return r.ctx
}

// Data returns information about the request.
func (r *Request) Data() *encore.Request {
	return r.cache.Get()
}

// Response represents the API handler's response.
// It can be modified by each middleware to customize
// the API response.
//
// The Payload's type must not be changed: it is not permissible
// to return a payload with a different type than the API returns.
//
// If Err is non-nil it becomes serialized as the response.
//
// For Raw endpoints neither Payload nor Err are used.
type Response struct {
	// Payload is the API's response payload.
	// It is nil if the API handler returned an error or if
	// the API is a raw endpoint.
	//
	// The payload can be mutated by middleware but the type must not be changed;
	// it must always be of the same type as the API handler's return type.
	// Otherwise, an error is returned.
	Payload any

	// Err is the error returned from the API handler or another middleware.
	// It is written as output for non-raw endpoints.
	Err error

	// HTTPStatus is the HTTP status code the response is written with.
	//
	// If zero is returned from middleware, Encore will choose an appropriate status code based
	// status code depending on the type of error being returned (or 200 for success).
	//
	// If a non-zero value is returned from a middleware, Encore will use that status code
	// regardless of what Err is set to.
	//
	// For raw handlers middleware cannot modify this as it has already
	// been written to the network.
	HTTPStatus int
}

// NewRequest constructs a new Request that returns the given context and request data.
// It is primarily used for testing middleware.
func NewRequest(ctx context.Context, data *encore.Request) Request {
	return Request{
		ctx:   ctx,
		cache: newReqCache(func() *encore.Request { return data }),
	}
}
