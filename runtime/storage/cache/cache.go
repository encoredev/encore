package cache

import (
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
)

type ClusterConfig struct {
}

type constStr string

type Cluster struct {
	cl *redis.Client
}

type KeyspaceConfig struct {
	Path          constStr
	DefaultExpiry time.Duration

	//publicapigen:drop
	EncoreInternal_KeyMapper any
	//publicapigen:drop
	EncoreInternal_ValueMapper any
}

// Nil is the error value reported when a key is missing from the cache.
var Nil = errors.New("cache: nil")
