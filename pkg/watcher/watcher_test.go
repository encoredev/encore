package watcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestCloseWakesWaitForEvents verifies that Close releases a caller parked
// in WaitForEvents, rather than leaving it asleep on the condition forever.
func TestCloseWakesWaitForEvents(t *testing.T) {
	w, err := New("test-app")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.RecursivelyWatch(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	done := make(chan bool, 1)
	go func() {
		_, ok := w.WaitForEvents()
		done <- ok
	}()

	// Wait until the caller is genuinely parked on the condition, so the
	// test can't pass merely by observing an already-closed watcher. There
	// is no hook between WaitForEvents' stop-check and its Wait, so stack
	// inspection is the only way to detect this.
	waitUntilParked(t)

	_ = w.Close()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("WaitForEvents reported events after Close, want ok=false")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForEvents did not return after Close; its caller is leaked")
	}
}

// waitUntilParked blocks until a goroutine is parked on the condition from
// inside WaitForEvents. It matches both frames in the same goroutine's
// stack, so an unrelated goroutine waiting on some other condition doesn't
// count.
func waitUntilParked(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	buf := make([]byte, 1<<20)
	for time.Now().Before(deadline) {
		n := runtime.Stack(buf, true)
		for _, g := range strings.Split(string(buf[:n]), "\n\n") {
			if strings.Contains(g, "sync.(*Cond).Wait") &&
				strings.Contains(g, "(*Watcher).WaitForEvents") {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a caller to park in WaitForEvents")
}

// TestFilesInNewDirectory verifies that files created together with a new
// directory are reported, even though they exist before the directory is watched.
// The directory is populated elsewhere and moved in, so the file never gets
// an event of its own.
func TestFilesInNewDirectory(t *testing.T) {
	w, err := New("test-app")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := w.RecursivelyWatch(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	staging := t.TempDir()
	staged := filepath.Join(staging, "a", "migrations", "1_init.up.sql")
	if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("CREATE TABLE t (id INT);"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(staging, "a"), filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "a", "migrations", "1_init.up.sql")

	found := make(chan struct{})
	go func() {
		for {
			events, ok := w.WaitForEvents()
			if !ok {
				return
			}
			for _, ev := range events {
				if ev.Path == file && ev.EventType == CREATED {
					close(found)
					return
				}
			}
		}
	}()

	select {
	case <-found:
	case <-time.After(5 * time.Second):
		t.Fatal("no event for a file created together with its directory")
	}
}
