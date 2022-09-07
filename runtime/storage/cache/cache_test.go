package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"encore.dev/appruntime/config"
)

func TestBasicKeyspace(t *testing.T) {
	kt := newCacheTest(t)
	ks, ctx := kt.ks, kt.ctx

	kt.Set("one", "alpha")
	kt.Val("one", "alpha")

	kt.Set("one", "beta", ExpireIn(time.Second))
	kt.Val("one", "beta")
	kt.TTL("one", time.Second)

	kt.Set("one", "charlie", ExpireIn(-time.Second))
	kt.Missing("one")

	kt.Set("one", "delta")
	kt.Val("one", "delta")
	check(ks.Delete(ctx, "one"))
	kt.Missing("one")

	// Replace should fail if the key is missing
	if err := ks.Replace(ctx, "one", "test"); err != Nil {
		t.Errorf("replace: unexpected error: %v", err)
	}
	check(ks.Add(ctx, "one", "added"))

	// Add should fail if the key is already present.
	if err := ks.Add(ctx, "one", "added twice"); err != Nil {
		t.Errorf("add: unexpected error: %v", err)
	}
	kt.Val("one", "added")

	check(ks.Replace(ctx, "one", "replaced"))
	kt.Val("one", "replaced")

	old := must(ks.GetAndSet(ctx, "one", "updated"))
	if old == nil || *old != "replaced" {
		t.Errorf("get and set: want old value %q, got %v", "replaced", old)
	}

	old = must(ks.GetAndDelete(ctx, "one"))
	if old == nil || *old != "updated" {
		t.Errorf("get and delete: want old value %q, got %v", "updated", old)
	}
	kt.Missing("one")
}

func TestStringKeyspace(t *testing.T) {
	kt := newCacheTest(t)
	ks, ctx := kt.ks, kt.ctx

	kt.Set("one", "alpha")

	newLen := must(ks.Append(ctx, "one", " bravo"))
	want := "alpha bravo"
	kt.Val("one", want)
	if newLen != int64(len(want)) {
		t.Errorf("append: got resulting length %d, want %d", newLen, len(want))
	}
}

func newCacheTest(t *testing.T) *ksTester {
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

	ks := NewStringKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper:   func(s string) string { return s },
		EncoreInternal_ValueMapper: func(s string) (string, error) { return s, nil },
	})

	ctx := context.Background()
	return &ksTester{t: t, ctx: ctx, ks: ks, srv: srv}
}

type ksTester struct {
	t   *testing.T
	ctx context.Context
	ks  *StringKeyspace[string]
	srv *miniredis.Miniredis
}

func (t *ksTester) Set(key, val string, opts ...WriteOption) {
	t.t.Helper()
	if err := t.ks.Set(t.ctx, key, val, opts...); err != nil {
		t.t.Errorf("Set(%q, %q) = %v, want nil", key, val, err)
	}
}

func (t *ksTester) Val(key, want string) {
	t.t.Helper()
	if got, err := t.ks.Get(t.ctx, key); err != nil {
		if err == Nil {
			t.t.Errorf("key %s: key not in cache", key)
		} else {
			t.t.Errorf("key %s: got err %v, want nil", key, err)
		}
	} else if got != want {
		t.t.Errorf("key %s: got value %q, want %q", key, got, want)
	}
}

func (t *ksTester) TTL(key string, want time.Duration) {
	t.t.Helper()
	if !t.srv.Exists(key) {
		t.t.Errorf("key %s: key not in cache", key)
	}
	got := t.srv.TTL(key)
	if got != want {
		t.t.Errorf("key %s: got ttl %v, want %v", key, got, want)
	}
}

func (t *ksTester) Missing(key string) {
	t.t.Helper()
	if _, err := t.ks.Get(t.ctx, key); err == nil {
		t.t.Errorf("key %s: key still in cache", key)
	} else if err != Nil {
		t.t.Errorf("key %s: got err %v, want Nil", key, err)
	}
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
