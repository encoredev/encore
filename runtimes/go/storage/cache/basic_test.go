package cache

import (
	"context"
	"errors"
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
	must(ks.Delete(ctx, "one"))
	kt.Missing("one")

	// Replace should fail if the key is missing
	if err := ks.Replace(ctx, "one", "test"); !errors.Is(err, Miss) {
		t.Errorf("replace: unexpected error: %v", err)
	}
	check(ks.SetIfNotExists(ctx, "one", "added"))

	// Add should fail if the key is already present.
	if err := ks.SetIfNotExists(ctx, "one", "added twice"); !errors.Is(err, KeyExists) {
		t.Errorf("add: unexpected error: %v", err)
	}
	kt.Val("one", "added")

	check(ks.Replace(ctx, "one", "replaced"))
	kt.Val("one", "replaced")

	old := must(ks.GetAndSet(ctx, "one", "updated"))
	if old != "replaced" {
		t.Errorf("get and set: want old value %q, got %v", "replaced", old)
	}

	old = must(ks.GetAndDelete(ctx, "one"))
	if old != "updated" {
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

	newLen = must(ks.SetRange(ctx, "one", 6, "ch"))
	want = "alpha chavo"
	kt.Val("one", want)
	if newLen != int64(len(want)) {
		t.Errorf("setrange: got resulting length %d, want %d", newLen, len(want))
	}

	newLen = must(ks.SetRange(ctx, "one", 13, "pad"))
	want = "alpha chavo\x00\x00pad"
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
		t.Errorf("set/get: got %v, want %v", got, want)
	}

	if got, want := must(ks.Increment(ctx, "one", 3)), int64(4); got != want {
		t.Errorf("incr: got %v, want %v", got, want)
	}

	if got, want := must(ks.Decrement(ctx, "one", 1)), int64(3); got != want {
		t.Errorf("decr: got %v, want %v", got, want)
	}
}

func TestFloatKeyspace(t *testing.T) {
	ks := newFloatTest(t)
	ctx := context.Background()

	// We may need to change these to approximate comparisons, but it's
	// at least passing for me right now.
	check(ks.Set(ctx, "one", 1))
	if got, want := must(ks.Get(ctx, "one")), float64(1); got != want {
		t.Errorf("set/get: got %v, want %v", got, want)
	}

	if got, want := must(ks.Increment(ctx, "one", 3)), float64(4); got != want {
		t.Errorf("incr: got %v, want %v", got, want)
	}

	if got, want := must(ks.Decrement(ctx, "one", 1)), float64(3); got != want {
		t.Errorf("decr: got %v, want %v", got, want)
	}
}

func TestMultiGet(t *testing.T) {
	kt := newStringTest(t)
	ks, ctx := kt.ks, kt.ctx

	// Set up test data
	kt.Set("key1", "value1")
	kt.Set("key2", "value2")

	results := must(ks.MultiGet(ctx, "key1", "key2", "missing"))
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Err != nil || results[0].Value != "value1" {
		t.Errorf("key1: got Err=%v, Value=%q, want Err=nil, Value=%q", results[0].Err, results[0].Value, "value1")
	}
	if results[1].Err != nil || results[1].Value != "value2" {
		t.Errorf("key2: got Err=%v, Value=%q, want Err=nil, Value=%q", results[1].Err, results[1].Value, "value2")
	}
	if !errors.Is(results[2].Err, Miss) {
		t.Errorf("missing: got Err=%v, want Miss", results[2].Err)
	}
}

func newStringTest(t *testing.T) *stringTester {
	cluster, srv := newTestCluster(t)
	ks := NewStringKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper: func(s string) string { return s },
	})

	ctx := context.Background()
	return &stringTester{t: t, ctx: ctx, ks: ks, srv: srv}
}

func newIntTest(t *testing.T) *IntKeyspace[string] {
	cluster, _ := newTestCluster(t)
	ks := NewIntKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper: func(s string) string { return s },
	})
	return ks
}

func newFloatTest(t *testing.T) *FloatKeyspace[string] {
	cluster, _ := newTestCluster(t)
	ks := NewFloatKeyspace[string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper: func(s string) string { return s },
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
		if err == Miss {
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
	} else if !errors.Is(err, Miss) {
		t.t.Errorf("key %s: got err %v, want Miss", key, err)
	}
}
