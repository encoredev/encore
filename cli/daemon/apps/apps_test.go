package apps

import (
	"path/filepath"
	"testing"

	"encr.dev/internal/conf"
	"encr.dev/pkg/watcher"
)

func newTestInstance(t *testing.T) *Instance {
	t.Helper()
	// Force off so the watcher doesn't also watch the runtime source tree,
	// which may not exist in the test environment.
	prev := conf.DevDaemon
	conf.DevDaemon = false
	t.Cleanup(func() { conf.DevDaemon = prev })

	i := NewInstance(t.TempDir(), "test-local-id", "")
	t.Cleanup(func() { _ = i.Close() })
	return i
}

func noopWatch(*Instance, []watcher.Event) {}

// TestUnwatchUnknownID verifies that Unwatch calls without a matching live
// subscription (double-unwatch, never-issued id) don't affect the watcher
// or other subscriptions.
func TestUnwatchUnknownID(t *testing.T) {
	i := newTestInstance(t)

	id1, err := i.Watch(noopWatch)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := i.Watch(noopWatch)
	if err != nil {
		t.Fatal(err)
	}

	i.Unwatch(id1)
	i.Unwatch(id1)   // double-unwatch
	i.Unwatch(99999) // never issued
	if !i.isWatching() {
		t.Fatal("watcher stopped while a subscription was still active")
	}

	i.Unwatch(id2)
	if i.isWatching() {
		t.Fatal("watcher still running after the last subscription was removed")
	}
}

// TestWatcherRestarts verifies the watcher can start and stop repeatedly,
// e.g. across successive `encore run` invocations.
func TestWatcherRestarts(t *testing.T) {
	i := newTestInstance(t)

	for n := 0; n < 3; n++ {
		id, err := i.Watch(noopWatch)
		if err != nil {
			t.Fatal(err)
		}
		if !i.isWatching() {
			t.Fatalf("cycle %d: watcher not running after Watch", n)
		}
		i.Unwatch(id)
		if i.isWatching() {
			t.Fatalf("cycle %d: watcher still running after Unwatch", n)
		}
	}
}

// TestCloseWithActiveSubscriptions verifies Close stops the watcher and that
// late Unwatch calls for subscriptions it cleared are no-ops.
func TestCloseWithActiveSubscriptions(t *testing.T) {
	i := newTestInstance(t)

	id1, err := i.Watch(noopWatch)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := i.Watch(noopWatch)
	if err != nil {
		t.Fatal(err)
	}

	if err := i.Close(); err != nil {
		t.Fatal(err)
	}
	if i.isWatching() {
		t.Fatal("watcher still running after Close")
	}

	i.Unwatch(id1)
	i.Unwatch(id2)
	if i.isWatching() {
		t.Fatal("watcher running after Unwatch of closed subscriptions")
	}
}

// TestWatchFailure verifies a failed Watch leaves no subscription and no
// running watcher behind.
func TestWatchFailure(t *testing.T) {
	i := newTestInstance(t)
	i.root = filepath.Join(i.root, "does-not-exist")

	for n := 0; n < 2; n++ {
		if _, err := i.Watch(noopWatch); err == nil {
			t.Fatalf("attempt %d: Watch succeeded on a nonexistent root", n)
		}
		if i.isWatching() {
			t.Fatalf("attempt %d: watcher running after failed Watch", n)
		}
	}
}
