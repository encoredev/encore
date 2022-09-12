package cache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"encore.dev/appruntime/config"
)

func newTestCluster(t *testing.T) (*Cluster, *miniredis.Miniredis) {
	srv := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	mgr := &Manager{
		cfg: &config.Config{Static: &config.Static{
			// We're testing the "production mode" of the cache, not the test mode.
			Testing: false,
		}},
	}
	cluster := &Cluster{
		mgr: mgr,
		cl:  redisClient,
	}
	return cluster, srv
}

func must[T any](val T, err error) T {
	check(err)
	return val
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
