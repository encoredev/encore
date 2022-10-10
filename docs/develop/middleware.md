---
title: Middleware
subtitle: Handling cross-cutting, generic functionality
---

Middleware is a way to write reusable code that runs before or after (or both)
the handling of API requests, often across several (or all) API endpoints.

It's commonly used to implement cross-cutting concerns like
[request logging](/docs/observability/logging),
[authentication](/docs/develop/auth),
[tracing](/docs/observability/tracing),
and so on. One of the benefits of Encore is that
all of these use cases are already handled out-of-the-box, so there's no
need to use middleware for those things.

Nonetheless, there are several use cases where it can be useful to write
reusable functionality that applies to multiple API endpoints, and middleware
is a good solution in those cases.

Encore provides built-in support for middleware by defining a function with the
`//encore:middleware` directive. The middleware directive takes a `target`
parameter that specifies which API endpoints it applies to.

## Middleware functions

A typical middleware implementation looks like this:

```go
import (
    "encore.dev/beta/errs"
    "encore.dev/middleware"
)

//encore:middleware global target=all
func ValidationMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
    // If the payload has a Validate method, use it to validate the request.
    payload := req.Data().Payload
    if validator, ok := payload.(interface { Validate() error }); ok {
        if err := validator.Validate(); err != nil {
            // If the validation fails, return an InvalidArgument error.
            err = errs.WrapCode(err, errs.InvalidArgument, "validation failed")
            return middleware.Response{Err: err}
        }
    }
    return next(req)
}
```

Middleware forms a chain, allowing each middleware to introspect and process
the incoming request before handing it off to the next middleware by calling the
`next` function that's passed in as an argument. For the last middleware in the
chain, calling `next` results in the actual API handler being called.

The `req` parameter provides information about the incoming request
(see [package docs](https://pkg.go.dev/encore.dev/middleware#Request)).

The `next` function returns a [`middleware.Response`](https://pkg.go.dev/encore.dev/middleware#Response)
object which contains the response from the API, describing whether there was an error, and on success
the actual response payload.

This enables middleware to also introspect and even
modify the outgoing response, like this:

```go
//encore:middleware target=tag:cache
func CachingMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
    data := req.Data()
    // Check if we have the response cached. Use the request path as the cache key.
    cacheKey := data.Path
	if cached, err := loadFromCache(cacheKey, data.API.ResponseType); err == nil && cached != nil {
	    return middleware.Response{Payload: cached}
    }
	// Otherwise forward the request to the handler
	return next(req)
}
```

This uses `target=tag:cache` to have the middleware only apply to APIs that have
that tag. More on this below in [Targeting APIs](#targeting-apis).

<Callout type="important">

Middleware functions can also be defined as methods on a Dependency Injection
struct declared with `//encore:service`. For example:

```go
//encore:service
type Service struct{}

//encore:middleware target=all
func (s *Service) MyMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	// ...
}
```

See the [Dependency Injection](/docs/how-to/dependency-injection) docs for more information.

</Callout>

## Middleware ordering

Middleware can either be defined inside a service, in which case it only runs
for APIs within that service, or it can be defined as a `global` middleware,
in which case it applies to all services. For global middleware the `target`
directive still applies and enables you to easily match a subset of APIs.

<Callout type="important">

Global middleware always run before all service-specific middleware,
and then run in the order they are defined in the source code based on
file name lexicographic ordering.

</Callout>

To avoid surprises it's best to define all middleware in a file called
`middleware.go` in each service, and to create a single top-level package
to contain all global middleware.

## Targeting APIs

The `target` directive can either be provided as `target=all` (meaning it applies
to all APIs) or a list of tags, in the form `target=tag:foo,tag:bar`. Note that
these tags are evaluated with `OR`, meaning the middleware applies to an API if
the API has at least one of those tags.

APIs can be defined with tags by adding `tag:foo` at the end of the `//encore:api` directive:

```go
//encore:api public method=GET path=/user/:id tag:cache
func GetUser(ctx context.Context, id string) (*User, error) {
	// ...
}
```
