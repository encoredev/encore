package cache

import (
	"context"
	"reflect"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestListKeyspace(t *testing.T) {
	kt := newListTest(t)
	ks, ctx := kt.ks, kt.ctx

	if got, want := kt.PushLeft("one", "a"), int64(1); got != want {
		t.Errorf("lpush: got len %v, want %v", got, want)
	}
	kt.Val("one", "a")

	if got, want := kt.PushLeft("one", "b"), int64(2); got != want {
		t.Errorf("lpush: got len %v, want %v", got, want)
	}
	kt.Val("one", "b", "a")

	if got, want := kt.PushRight("one", "c"), int64(3); got != want {
		t.Errorf("rpush: got len %v, want %v", got, want)
	}
	kt.Val("one", "b", "a", "c")

	kt.Set("one", 1, "d")
	kt.Val("one", "b", "d", "c")

	kt.Trim("one", 1, 1)
	kt.Val("one", "d")

	if got, want := must(ks.InsertBefore(ctx, "one", "d", "e")), int64(2); got != want {
		t.Errorf("insertbefore: got len %v, want %v", got, want)
	}
	kt.Val("one", "e", "d")
	if got, want := must(ks.InsertAfter(ctx, "one", "e", "f")), int64(3); got != want {
		t.Errorf("insertafter: got len %v, want %v", got, want)
	}
	kt.Val("one", "e", "f", "d")

	kt.PushRight("one", "e")
	kt.PushRight("one", "e")
	kt.PushRight("one", "f")
	kt.Val("one", "e", "f", "d", "e", "e", "f")

	if got, want := must(ks.RemoveFirst(ctx, "one", 2, "e")), int64(2); got != want {
		t.Errorf("removefirst: got %v, want %v", got, want)
	}
	kt.Val("one", "f", "d", "e", "f")
	if got, want := must(ks.RemoveLast(ctx, "one", 1, "f")), int64(1); got != want {
		t.Errorf("removelast: got %v, want %v", got, want)
	}
	kt.Val("one", "f", "d", "e")

	if got, want := must(ks.Move(ctx, "one", "one", Left, Right)), "f"; got != want {
		t.Errorf("move: got %q, want %q", got, want)
	}
	kt.Val("one", "d", "e", "f")
}

func newListTest(t *testing.T) *listTester {
	cluster, srv := newTestCluster(t)
	ks := NewListKeyspace[string, string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper: func(s string) string { return s },
	})
	ctx := context.Background()
	return &listTester{t: t, ctx: ctx, ks: ks, srv: srv}
}

type listTester struct {
	t   *testing.T
	ctx context.Context
	ks  *ListKeyspace[string, string]
	srv *miniredis.Miniredis
}

func (t *listTester) PushLeft(key, val string) int64 {
	t.t.Helper()
	newLen, err := t.ks.PushLeft(t.ctx, key, val)
	if err != nil {
		t.t.Errorf("LPush(%q, %q) = %v, want nil", key, val, err)
	}
	return newLen
}

func (t *listTester) PushRight(key, val string) int64 {
	t.t.Helper()
	newLen, err := t.ks.PushRight(t.ctx, key, val)
	if err != nil {
		t.t.Errorf("RPush(%q, %q) = %v, want nil", key, val, err)
	}
	return newLen
}

func (t *listTester) Set(key string, idx int, val string) {
	t.t.Helper()
	err := t.ks.Set(t.ctx, key, int64(idx), val)
	if err != nil {
		t.t.Errorf("Set(%q, %v, %q) = %v, want nil", key, idx, val, err)
	}
}

func (t *listTester) Trim(key string, from, to int) {
	t.t.Helper()
	err := t.ks.Trim(t.ctx, key, int64(from), int64(to))
	if err != nil {
		t.t.Errorf("Trim(%q, %v, %v) = %v, want nil", key, from, to, err)
	}
}

func (t *listTester) Val(key string, want ...string) {
	t.t.Helper()
	got := must(t.ks.GetRange(t.ctx, key, 0, -1))
	if !reflect.DeepEqual(got, want) {
		t.t.Errorf("key %s: got %+v, want %+v", key, got, want)
	}
	num := must(t.ks.Len(t.ctx, key))
	if num != int64(len(got)) {
		t.t.Errorf("got len %d, expected %d (from getRange)", num, len(got))
	}
}
