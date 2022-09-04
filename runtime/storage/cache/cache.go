package cache

import (
	"errors"
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

	// DefaultExpiry specifies the default key expiry for cache items
	// in the cluster.
	//
	// Per-Keyspace configuration takes precedence over this field.
	//
	// If both Cluster and Keyspace have a nil DefaultExpiry,
	// values will be set without an expiry but may still be evicted
	// based on the EvictionPolicy.
	DefaultExpiry ExpiryFunc
}

// An EvictionPolicy describes how the cache evicts keys to make room for new data
// when the maximum memory limit is reached.
//
// See https://redis.io/docs/manual/eviction/#eviction-policies for more information.
type EvictionPolicy string

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
	VolatileLRU EvictionPolicy = "volatile-lfu"

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
	// in this keyspace. If nil the cluster default expiry is used.
	DefaultExpiry ExpiryFunc

	// EncoreInternal_KeyMapper specifies how typed keys are translated
	// to a string. It's of type any to avoid making KeyspaceConfig
	// a generic type. It's an internal field set by Encore's compiler.
	//publicapigen:drop
	EncoreInternal_KeyMapper any
	// EncoreInternal_ValueMapper specifies how Redis values are translated
	// to Go. It's of type any to avoid making KeyspaceConfig
	// a generic type. It's an internal field set by Encore's compiler.
	//publicapigen:drop
	EncoreInternal_ValueMapper any
}

// Nil is the error value reported when a key is missing from the cache.
var Nil = errors.New("cache: nil")

// ExpiryFunc is a function that reports when a key should expire
// given the current time.
type ExpiryFunc func(now time.Time) time.Time

// NeverExpire is a sentinel value indicating a key should never expire.
var NeverExpire = time.Unix(0, 1)

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
