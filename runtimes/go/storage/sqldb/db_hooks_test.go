package sqldb

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDatabaseAddHooksAppends(t *testing.T) {
	db := &Database{hooks: &hookList{}}

	firstCalled := false
	db.AddHooks(Hooks{AfterConnect: func(context.Context, *pgx.Conn) error {
		firstCalled = true
		return nil
	}})

	secondCalled := false
	db.AddHooks(Hooks{AfterConnect: func(context.Context, *pgx.Conn) error {
		secondCalled = true
		return nil
	}})

	list := db.hooks
	if list == nil {
		t.Fatal("expected hook list to be initialized")
	}
	list.mu.RLock()
	count := len(list.hooks)
	list.mu.RUnlock()
	if count != 2 {
		t.Fatalf("expected 2 hooks, got %d", count)
	}

	if err := db.hooks.runAfterConnectHooks(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error running hooks: %v", err)
	}
	if !firstCalled || !secondCalled {
		t.Fatalf("expected both hooks to be called, first=%v second=%v", firstCalled, secondCalled)
	}
}

func TestDatabaseRunAfterConnectHooksSkipsNilAndReturnsError(t *testing.T) {
	db := &Database{hooks: &hookList{}}
	wantErr := errors.New("boom")
	called := false

	db.AddHooks(Hooks{})
	db.AddHooks(Hooks{AfterConnect: func(context.Context, *pgx.Conn) error {
		called = true
		return wantErr
	}})

	err := db.hooks.runAfterConnectHooks(context.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if !called {
		t.Fatal("expected non-nil hook to be invoked")
	}
}
