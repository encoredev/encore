// Package cache provides the ability to define distributed Redis cache clusters
// and functionality to use them in a fully type-safe manner.
//
// For more information see https://encore.dev/docs/develop/caching
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// ClusterConfig represents the configuration of cache clusters.
type ClusterConfig struct {
	// EvictionPolicy decides how the cache evicts existing keys
	// to make room for new data when the max memory limit is reached.
	//
	// If not specified the cache defaults to AllKeysLRU.
	EvictionPolicy EvictionPolicy
}

// An EvictionPolicy describes how the cache evicts keys to make room for new data
// when the maximum memory limit is reached.
//
// See https://redis.io/docs/manual/eviction/#eviction-policies for more information.
type EvictionPolicy string

// NOTE: These values need to be added to the runtimeconstants package
// and the parser package for the parser to be aware of them.

// The eviction policies Encore supports.
// See https://redis.io/docs/manual/eviction/#eviction-policies for more information.
const (
	// AllKeysLRU keeps most recently used keys and removes least recently used (LRU) keys.
	// This is a good default choice for most cache use cases if you're not sure.
	AllKeysLRU EvictionPolicy = "allkeys-lru"

	// AllKeysLFU keeps frequently used keys and removes least frequently used (LFU) keys.
	AllKeysLFU EvictionPolicy = "allkeys-lfu"

	// AllKeysRandom randomly removes keys as needed.
	AllKeysRandom EvictionPolicy = "allkeys-random"

	// VolatileLRU removes least recently used keys with an expiration set.
	// It behaves like NoEviction if there are no keys to evict with an expiration set.
	VolatileLRU EvictionPolicy = "volatile-lru"

	// VolatileLFU removes least frequently used keys with an expiration set.
	// It behaves like NoEviction if there are no keys to evict with an expiration set.
	VolatileLFU EvictionPolicy = "volatile-lfu"

	// VolatileTTL removes keys with an expiration set and the shortest time to live (TTL).
	// It behaves like NoEviction if there are no keys to evict with an expiration set.
	VolatileTTL EvictionPolicy = "volatile-ttl"

	// VolatileRandom randomly removes keys with an expiration set.
	// It behaves like NoEviction if there are no keys to evict with an expiration set.
	VolatileRandom EvictionPolicy = "volatile-random"

	// NoEviction does not evict any keys, and instead returns an error to the client
	// when the max memory limit is reached.
	NoEviction EvictionPolicy = "noeviction"
)

//publicapigen:keep
type constStr string

// Cluster represents a Redis cache cluster.
type Cluster struct {
	cfg ClusterConfig
	mgr *Manager
	cl  *redis.Client
}

// KeyspaceConfig specifies the configuration options for a cache keyspace.
type KeyspaceConfig struct {
	// KeyPattern is a string literal representing the
	// cache key pattern for this keyspace.
	KeyPattern constStr

	// DefaultExpiry specifies the default key expiry for cache items
	// in this keyspace.
	//
	// When set, all write operations set (for new keys)
	// or update (for existing keys) the expiration time.
	//
	// When updating the expiration time Encore always
	// performs the combined operation atomically.
	//
	// If nil, cache items have no expiry date by default.
	//
	// The default behavior can be overridden by passing in
	// an ExpiryFunc or KeepTTL as a WriteOption to a specific operation.
	DefaultExpiry ExpiryFunc

	// EncoreInternal_DefLoc specifies where the keyspace is defined.
	// It's an internal field set by Encore's compiler.
	//publicapigen:drop
	EncoreInternal_DefLoc uint32

	// EncoreInternal_KeyMapper specifies how typed keys are translated
	// to a string. It's of type any to avoid making KeyspaceConfig
	// a generic type. It's an internal field set by Encore's compiler.
	//
	// The type must be func(K) string.
	//publicapigen:drop
	EncoreInternal_KeyMapper any
}

// An OpError describes the operation that failed.
type OpError struct {
	Operation string
	RawKey    string
	Err       error
}

func (e *OpError) Error() string {
	return fmt.Sprintf("cache: %s %q: %v", e.Operation, e.RawKey, e.Err)
}

func (e *OpError) Unwrap() error {
	return e.Err
}

// Miss is the error value reported when a key is missing from the cache.
// It must be checked against with errors.Is.
var Miss = errors.New("cache miss")

// KeyExists is the error reported when a key already exists
// and the requested operation is specified to only apply to
// keys that do not already exist.
// It must be checked against with errors.Is.
var KeyExists = errors.New("key already exists")

// Result represents the result of a cache operation that may or may not have found a value.
// If Err is nil, Value contains the cached value.
// If Err matches Miss, the key was not found in the cache.
// Otherwise Err contains the error that occurred.
type Result[V any] struct {
	// Value holds the cached value if Err is nil, otherwise the zero value.
	Value V
	// Err is nil on success, Miss if the key was not found, or another error.
	Err error
}

// An WriteOption customizes the behavior of a single cache write operation.
type WriteOption interface {
	//publicapigen:keep
	writeOption() // ensure only our package can implement
}

type expiryOption interface {
	WriteOption
	expiry(now time.Time) time.Time
}

// ExpiryFunc is a function that reports when a key should expire
// given the current time. It can be used as a WriteOption to customize
// the expiration for that particular operation.
type ExpiryFunc func(now time.Time) time.Time

// option implements WriteOption.
//
//publicapigen:keep
func (ExpiryFunc) writeOption() {}

// expiry implements expiryOption.
func (fn ExpiryFunc) expiry(now time.Time) time.Time {
	return fn(now)
}

var _ expiryOption = (ExpiryFunc)(nil)

// ExpireIn returns an ExpiryFunc that expires keys after a constant duration.
func ExpireIn(dur time.Duration) ExpiryFunc {
	return func(now time.Time) time.Time { return now.Add(dur) }
}

// ExpireDailyAt returns an ExpiryFunc that expires keys daily at the given time of day in loc.
// ExpireDailyAt panics if loc is nil.
func ExpireDailyAt(hour, minute, second int, loc *time.Location) ExpiryFunc {
	return func(now time.Time) time.Time {
		year, month, day := now.Date()
		next := time.Date(year, month, day, hour, minute, second, 0, loc)
		// If next has already passed today, move it to tomorrow.
		if next.Before(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}
}

// expiryTime is a type for time constants that are also WriteOptions.
//
//publicapigen:keep
type expiryTime time.Time

//publicapigen:keep
func (expiryTime) writeOption() {}

func (et expiryTime) expiry(_ time.Time) time.Time {
	return time.Time(et)
}

var _ expiryOption = (expiryTime)(time.Time{})

var (
	// NeverExpire is a WriteOption indicating the key should never expire.
	NeverExpire = expiryTime(neverExpire)

	// KeepTTL is a WriteOption indicating the key's TTL should be kept the same.
	KeepTTL = expiryTime(keepTTL)
)

//publicapigen:keep
var (
	neverExpire = time.Unix(0, 1)
	keepTTL     = time.Unix(0, 2)
)

func fnMap[A, B any](src []A, fn func(A) B) []B {
	dst := make([]B, len(src))
	for i, v := range src {
		dst[i] = fn(v)
	}
	return dst
}

// do executes a Redis command, by invoking fn.
//
// Depending on the keyspace's expiration logic it either calls fn with
// a plain Redis client or with a transactional pipeline, in order to ensure
// the command executes atomically with the expiration update.
func do[K, V, Res any](cl *client[K, V], ctx context.Context, key string, fn func(cmdable) Res) Res {
	exp := cl.expiryCmd(ctx, key)
	if exp == nil {
		return fn(cl.redis)
	}

	pipe := cl.redis.TxPipeline()
	res := fn(pipe)
	_ = pipe.Process(ctx, exp)
	_, _ = pipe.Exec(ctx)
	return res
}

// do2 is like do, except it operates on two keys instead of one.
func do2[K, V, Res any](
	cl *client[K, V],
	ctx context.Context,
	keyA, keyB string,
	fn func(cmdable) Res,
) Res {
	expA := cl.expiryCmd(ctx, keyA)
	expB := cl.expiryCmd(ctx, keyB)

	// If we don't have any expiry commands, process the command directly.
	if expA == nil && expB == nil {
		return fn(cl.redis)
	}

	// Otherwise use a pipeline.
	pipe := cl.redis.TxPipeline()
	res := fn(pipe)
	if expA != nil {
		_ = pipe.Process(ctx, expA)
	}
	if expB != nil {
		_ = pipe.Process(ctx, expB)
	}
	_, _ = pipe.Exec(ctx)

	return res
}

type cmdable interface {
	redis.Cmdable
	Process(context.Context, redis.Cmder) error
}
