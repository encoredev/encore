---
seotitle: Using caches in your microservices backend application
seodesc: Learn how to implement caches to optimize response times and reduce cost in your microservices cloud backend.
title: Caching
subtitle: Optimize response times and reduce costs by avoiding re-work
infobox: {
  title: "Caching",
  import: "encore.dev/storage/cache",
}
---

A cache is a high-speed storage layer, commonly used in distributed systems to improve user experiences
by reducing latency, improving system performance, and avoiding expensive computation.

For scalable systems you typically want to deploy the cache as a separate
infrastructure resource, allowing you to run multiple instances of your application concurrently.

Encore's built-in Caching API lets you use high-performance caches (using [Redis](https://redis.io/)) in a cloud-agnostic declarative fashion. At deployment, Encore will automatically [provision the required infrastructure](/docs/deploy/infra).

## Cache clusters

To use caching in Encore, you must first define a *cache cluster*.
Each cache cluster defined in your application will be provisioned as a separate Redis instance
by Encore.

This gives you fine-grained control over which service(s) should use the same cache cluster
and which should have a separate one.

It looks like this:

```go
import "encore.dev/storage/cache"

var MyCacheCluster = cache.NewCluster("my-cache-cluster", cache.ClusterConfig{
    // EvictionPolicy tells Redis how to evict keys when the cache reaches
    // its memory limit. For typical cache use cases, cache.AllKeysLRU is a good default.
    EvictionPolicy: cache.AllKeysLRU,
})
```

<Callout type="info">

When starting out it's recommended to use a single cache cluster
that's shared between your different services.

</Callout>

## Keyspaces

When using a cache, each cached item is stored at a particular key, which is typically an arbitrary string.
If you use a cache cluster to cache different sets of data, it's important that distinct data set have non-overlapping keys.

Each value stored in the cache also has a specific type, and certain cache operations can only be performed on certain types. For example, a common cache operation is to increment an integer value that is stored in the cache. If you try to apply this operation on a value that is not an integer, an error is returned.

Encore provides a simple, type-safe solution to these problems through Keyspaces.

In order to begin storing data in your cache, you must first define a Keyspace.

Each keyspace has a Key type and a Value type. The Key type is much like a map key, in that it tells Encore where in the cache
the item is stored. The Key type is combined with the Key Pattern to produce a string that is the Redis cache key.

The Value type is the type of the values stored in that keyspace. For many keyspaces this is specified in the name of the constructor.
For example, `NewIntKeyspace` stores `int64` values.

For example, if you want to rate limit the number of requests per user ID it looks like this:

```go
import (
    "encore.dev/beta/auth"
    "encore.dev/beta/errs"
    "encore.dev/middleware"
)

// RequestsPerUser tracks the number of requests per user.
// The cache items expire after 10 seconds without activity.
var RequestsPerUser = cache.NewIntKeyspace[auth.UID](cluster, cache.KeyspaceConfig{
	KeyPattern:    "requests/:key",
	DefaultExpiry: cache.ExpireIn(10 * time.Second),
})

// RateLimitMiddleware is a global middleware that limits the number of authenticated requests
// to 10 requests per 10 seconds.
//encore:middleware target=all
func RateLimitMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	if userID, ok := auth.UserID(); ok {
		val, err := RequestsPerUser.Increment(req.Context(), userID, 1)

		// NOTE: this "fails open", meaning if we can't communicate with the cache
		// we default to allowing the requests.
		//
		// Consider whether that's the correct behavior for your application,
		// or if you want to return an error to the user in that case.
		if err == nil && val > 10 {
			return middleware.Response{
				Err: &errs.Error{Code: errs.ResourceExhausted, Message: "rate limit exceeded"},
			}
		}
	}
	return next(req)
}
```

As you can see, the `RequestsPerUser` defines a `KeyPattern` which is set to `"requests/:key"`.
Here `:key` refers to the value of the Key type, which is the `auth.UID` value passed in.

If you want the cache key to contain multiple values, you can define a struct type
and pass that as the key. Then change the `KeyPattern` to specify the struct fields.

For example:

```go
type MyKey struct {
    UserID auth.UID
    ResourcePath string // the resource being accessed
}

// ResourceRequestsPerUser tracks the number of requests per user and resource.
// The cache items expire after 10 seconds without activity.
var ResourceRequestsPerUser = cache.NewIntKeyspace[MyKey](cluster, cache.KeyspaceConfig{
	KeyPattern:    "requests/:UserID/:ResourcePath",
	DefaultExpiry: cache.ExpireIn(10 * time.Second),
})

// ... then:
key := MyKey{UserID: "some-user-id", ResourcePath: "/foo"}
ResourceRequestsPerUser.Increment(ctx, key, 1)
```

<Callout type="info">

Encore ensures that all the struct fields are present in the `KeyPattern`,
and that the placeholder values are all valid field names.

That way the connection between the struct fields and the `KeyPattern`
become compile-time type-safe as well.

</Callout>

Also note that Encore ensures there are no conflicting `KeyPattern` definitions across each cache cluster.
Each keyspace must define its own, non-conflicting `KeyPattern`.
This way, you can feel safe that there won't be any accidental overwrites of cache values, even with multiple services sharing the same cache cluster.

## Keyspace operations

Encore comes with a full suite of keyspace types, each with a wide variety of cache operations.

Basic keyspace types include
[strings](https://pkg.go.dev/encore.dev/storage/cache#NewStringKeyspace),
[integers](https://pkg.go.dev/encore.dev/storage/cache#NewIntKeyspace),
[floats](https://pkg.go.dev/encore.dev/storage/cache#NewFloatKeyspace),
and [struct types](https://pkg.go.dev/encore.dev/storage/cache#NewStructKeyspace).
These keyspaces all share the same set of methods (along with a few keyspace-specific ones).

There are also more advanced keyspaces for storing [sets of basic types](https://pkg.go.dev/encore.dev/storage/cache#NewSetKeyspace)
and [ordered lists of basic types](https://pkg.go.dev/encore.dev/storage/cache#NewListKeyspace).
These keyspaces offer a different, specialized set of methods specific to set and list operations.

For a list of the supported operations, see the [package documentation](https://pkg.go.dev/encore.dev/storage/cache).

## Testing

When running tests, Encore spins up an in-memory cache separately for each test.

This way you don't have to think about clearing the cache between tests,
or worrying about whether one test affects another.
Each test is automatically fully isolated.

## Local development

For local development, Encore maintains a local, in-memory implementation of Redis.
This implementation is designed to store a small amount of keys (currently 100).

When the number of keys exceeds this value, keys are randomly purged to get below the limit.
This is designed in order to simulate the ephemeral, transient nature of caches while also
limiting memory use. The precise behavior for local development may change over time and should not be relied on.
