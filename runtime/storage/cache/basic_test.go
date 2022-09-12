package cache

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestBasicKeyspace(t *testing.T) {
	kt := newStringTest(t)
	ks, ctx := kt.ks, kt.ctx

	kt.Set("one", "alpha")
	kt.Val("one", "alpha")

	check(ks.With(ExpireIn(time.Second)).Set(ctx, "one", "beta"))
	kt.Val("one", "beta")
	kt.TTL("one", time.Second)

	check(ks.With(ExpireIn(-time.Second)).Set(ctx, "one", "charlie"))
	kt.Missing("one")

	kt.Set("one", "delta")
	kt.Val("one", "delta")
	check(ks.Delete(ctx, "one"))
	kt.Missing("one")

	// Replace should fail if the key is missing
	if err := ks.Replace(ctx, "one", "test"); err != Nil {
		t.Errorf("replace: unexpected error: %v", err)
	}
	check(ks.SetIfNotExists(ctx, "one", "added"))

	// Add should fail if the key is already present.
	if err := ks.SetIfNotExists(ctx, "one", "added twice"); err != Nil {
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
	kt := newStringTest(t)
	ks, ctx := kt.ks, kt.ctx

	kt.Set("one", "alpha")

	newLen := must(ks.Append(ctx, "one", " bravo"))
	want := "alpha bravo"
	kt.Val("one", want)
	if newLen != int64(len(want)) {
		t.Errorf("append: got resulting length %d, want %d", newLen, len(want))
	}

	newLen = must(ks.SetRange(ctx, "one", 6, "charlie"))
	want = "alpha charlie"
	kt.Val("one", want)
	if newLen != int64(len(want)) {
		t.Errorf("setrange: got resulting length %d, want %d", newLen, len(want))
	}

	got := must(ks.GetRange(ctx, "one", 2, 4))
	if want := "pha"; got != want {
		t.Errorf("getrange: got %q, want %q", got, want)
	}

	gotLen := int(must(ks.Len(ctx, "one")))
	if want := len(must(ks.Get(ctx, "one"))); gotLen != want {
		t.Errorf("len: got %v, want %v", got, want)
	}
}

func TestIntKeyspace(t *testing.T) {
	ks := newIntTest(t)
	ctx := context.Background()

	check(ks.Set(ctx, "one", 1))
	if got, want := must(ks.Get(ctx, "one")), int64(1); got != want {
		t.Errorf("set/get: got val %v, want %v", got, want)
	}

	if got, want := must(ks.Incr(ctx, "one", 3)), int64(4); got != want {
		t.Errorf("incr: got val %v, want %v", got, want)
	}

	if got, want := must(ks.Decr(ctx, "one", 1)), int64(3); got != want {
		t.Errorf("decr: got val %v, want %v", got, want)
	}
}

func TestFloatKeyspace(t *testing.T) {
	ks := newFloatTest(t)
	ctx := context.Background()

	// We may need to change these to approximate comparisons, but it's
	// at least passing for me right now.
	check(ks.Set(ctx, "one", 1))
	if got, want := must(ks.Get(ctx, "one")), float64(1); got != want {
		t.Errorf("set/get: got val %v, want %v", got, want)
	}

	if got, want := must(ks.Incr(ctx, "one", 3)), float64(4); got != want {
		t.Errorf("incr: got val %v, want %v", got, want)
	}

	if got, want := must(ks.Decr(ctx, "one", 1)), float64(3); got != want {
		t.Errorf("decr: got val %v, want %v", got, want)
	}
}

func newStringTest(t *testing.T) *stringTester {
	cluster, srv := newTestCluster(t)
	ks := NewStringKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper:   func(s string) string { return s },
		EncoreInternal_ValueMapper: func(s string) (string, error) { return s, nil },
	})

	ctx := context.Background()
	return &stringTester{t: t, ctx: ctx, ks: ks, srv: srv}
}

func newIntTest(t *testing.T) *IntKeyspace[string] {
	cluster, _ := newTestCluster(t)
	ks := NewIntKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper:   func(s string) string { return s },
		EncoreInternal_ValueMapper: func(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) },
	})
	return ks
}

func newFloatTest(t *testing.T) *FloatKeyspace[string] {
	cluster, _ := newTestCluster(t)
	ks := NewFloatKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper:   func(s string) string { return s },
		EncoreInternal_ValueMapper: func(s string) (float64, error) { return strconv.ParseFloat(s, 64) },
	})
	return ks
}

type stringTester struct {
	t   *testing.T
	ctx context.Context
	ks  *StringKeyspace[string]
	srv *miniredis.Miniredis
}

func (t *stringTester) Set(key, val string) {
	t.t.Helper()
	if err := t.ks.Set(t.ctx, key, val); err != nil {
		t.t.Errorf("Set(%q, %q) = %v, want nil", key, val, err)
	}
}

func (t *stringTester) Val(key, want string) {
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

func (t *stringTester) TTL(key string, want time.Duration) {
	t.t.Helper()
	if !t.srv.Exists(key) {
		t.t.Errorf("key %s: key not in cache", key)
	}
	got := t.srv.TTL(key)
	if got != want {
		t.t.Errorf("key %s: got ttl %v, want %v", key, got, want)
	}
}

func (t *stringTester) Missing(key string) {
	t.t.Helper()
	if _, err := t.ks.Get(t.ctx, key); err == nil {
		t.t.Errorf("key %s: key still in cache", key)
	} else if err != Nil {
		t.t.Errorf("key %s: got err %v, want Nil", key, err)
	}
}
