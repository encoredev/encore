package cache

import (
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/shared/reqtrack"
)

func newTestCluster(t *testing.T) (*Cluster, *miniredis.Miniredis) {
	srv := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	rt := reqtrack.New(zerolog.New(os.Stdout), nil, nil)
	mgr := &Manager{
		static: &config.Static{
			// We're testing the "production mode" of the cache, not the test mode.
			Testing: false,
		},
		rt: rt,
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
