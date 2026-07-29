package watcher

import (
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
