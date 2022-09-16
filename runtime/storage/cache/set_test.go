package cache

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestSets(t *testing.T) {
	kt := newSetTest(t)
	ks, ctx := kt.ks, kt.ctx

	if got, want := kt.Add("one", "a", "b"), 2; got != want {
		t.Errorf("Add() = %d, want %d", got, want)
	}
	kt.Val("one", "a", "b")
	if got, want := kt.Remove("one", "b", "c"), 1; got != want {
		t.Errorf("Remove() = %d, want %d", got, want)
	}
	kt.Val("one", "a")

	{
		got := must(ks.SampleWithReplacement(ctx, "one", 3))
		checkSorted(t, got, "a", "a", "a")
	}

	if got, want := must(ks.SampleOne(ctx, "one")), "a"; got != want {
		t.Errorf("SampleOne: got %v, want %v", got, want)
	}
	if got, want := must(ks.PopOne(ctx, "one")), "a"; got != want {
		t.Errorf("PopOne: got %v, want %v", got, want)
	}
	if _, err := ks.SampleOne(ctx, "one"); !errors.Is(err, Miss) {
		t.Errorf("SampleOne: got err %v, want %v", err, Miss)
	}
	if _, err := ks.PopOne(ctx, "one"); !errors.Is(err, Miss) {
		t.Errorf("PopOne: got err %v, want %v", err, Miss)
	}

	kt.Add("one", "a", "b")
	kt.Contains("one", "a")
	kt.Missing("one", "c")
	if got, want := kt.Add("one", "a", "b", "c"), 1; got != want {
		t.Errorf("Add() = %d, want %d", got, want)
	}

	checkSetMap(t, must(ks.ItemsMap(ctx, "one")), "a", "b", "c")

	if got, want := must(ks.Len(ctx, "one")), int64(3); got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}

	{
		got := must(ks.Sample(ctx, "one", 3))
		checkSorted(t, got, "a", "b", "c")
	}

	{
		got := must(ks.Pop(ctx, "one", 3))
		checkSorted(t, got, "a", "b", "c")
	}

	{
		kt.Add("D1", "a", "b", "c")
		kt.Add("D2", "c", "d", "e")
		got := must(ks.Diff(ctx, "D1", "D2"))
		want := []string{"a", "b"}
		checkSorted(t, got, want...)
		checkSetMap(t, must(ks.DiffMap(ctx, "D1", "D2")), want...)
		if got, want := must(ks.DiffStore(ctx, "D3", "D1", "D2")), int64(len(want)); got != want {
			t.Errorf("DiffStore: got %v, want %v", got, want)
		}
		kt.Val("D3", want...)
	}

	{
		kt.Add("I1", "a", "b", "c")
		kt.Add("I2", "c", "d", "e")
		got := must(ks.Intersect(ctx, "I1", "I2"))
		want := []string{"c"}
		checkSorted(t, got, want...)
		checkSetMap(t, must(ks.IntersectMap(ctx, "I1", "I2")), want...)
		if got, want := must(ks.IntersectStore(ctx, "I3", "I1", "I2")), int64(len(want)); got != want {
			t.Errorf("IntersectStore: got %v, want %v", got, want)
		}
		kt.Val("I3", want...)
	}

	{
		kt.Add("U1", "a", "b", "c")
		kt.Add("U2", "c", "d", "e")
		got := must(ks.Union(ctx, "U1", "U2"))
		want := []string{"a", "b", "c", "d", "e"}
		checkSorted(t, got, want...)
		checkSetMap(t, must(ks.UnionMap(ctx, "U1", "U2")), want...)
		if got, want := must(ks.UnionStore(ctx, "U3", "U1", "U2")), int64(len(want)); got != want {
			t.Errorf("UnionStore: got %v, want %v", got, want)
		}
		kt.Val("U3", want...)
	}

	{
		kt.Add("M1", "a", "b", "c")
		kt.Add("M2", "a")

		must(ks.Move(ctx, "M1", "M2", "a"))
		kt.Val("M1", "b", "c")
		kt.Val("M2", "a")

		must(ks.Move(ctx, "M1", "M2", "b"))
		kt.Val("M1", "c")
		kt.Val("M2", "a", "b")
	}
}

func checkSorted(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func checkSetMap(t *testing.T, got map[string]struct{}, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("got %d items, want %d", len(got), len(want))
		return
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("wanted key %q, not found", w)
		}
	}
}

func newSetTest(t *testing.T) *setTester {
	cluster, srv := newTestCluster(t)
	ks := NewSetKeyspace[string, string](cluster, KeyspaceConfig{
		EncoreInternal_KeyMapper: func(s string) string { return s },
	})
	ctx := context.Background()
	return &setTester{t: t, ctx: ctx, ks: ks, srv: srv}
}

type setTester struct {
	t   *testing.T
	ctx context.Context
	ks  *SetKeyspace[string, string]
	srv *miniredis.Miniredis
}

func (t *setTester) Add(key string, val ...string) int {
	t.t.Helper()
	numAdded, err := t.ks.Add(t.ctx, key, val...)
	if err != nil {
		t.t.Errorf("Add(%q, %v+) = %v, want nil", key, val, err)
	}
	return numAdded
}

func (t *setTester) Remove(key string, val ...string) int {
	t.t.Helper()
	numRemoved, err := t.ks.Remove(t.ctx, key, val...)
	if err != nil {
		t.t.Errorf("Remove(%q, %+v) = %v, want nil", key, val, err)
	}
	return int(numRemoved)
}

func (t *setTester) Contains(key string, val string) {
	t.t.Helper()
	if ok, err := t.ks.Contains(t.ctx, key, val); err != nil {
		t.t.Errorf("Contains(%q, %q) = %v, want nil", key, val, err)
	} else if !ok {
		t.t.Errorf("key %q: value %q is missing in the set", key, val)
	}
}

func (t *setTester) Missing(key string, val string) {
	t.t.Helper()
	if ok, err := t.ks.Contains(t.ctx, key, val); err != nil {
		t.t.Errorf("Contains(%q, %q) = %v, want nil", key, val, err)
	} else if ok {
		t.t.Errorf("key %q: value %q is present in the set", key, val)
	}
}

func (t *setTester) Val(key string, want ...string) {
	t.t.Helper()
	got := must(t.ks.Items(t.ctx, key))
	sort.Strings(want)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.t.Errorf("key %s: got %+v, want %+v", key, got, want)
	}
}
