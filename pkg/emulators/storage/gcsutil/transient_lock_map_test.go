package gcsutil

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func (l *TransientLockMap) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.locks)
}

func TestTransientLockMapBasics(t *testing.T) {
	m := NewTransientLockMap()
	upstream := make(chan int, 1)
	downstream := make(chan int, 1)

	waitFor := func(c <-chan int, expected int) {
		val := <-c
		assert.Equal(t, expected, val, "got an unexpected value from a channel")
	}

	readSignalNow := func(c <-chan int, expected int, msg string) {
		select {
		case val := <-c:
			assert.Equal(t, expected, val, "got an unexpected value from a channel")
		default:
			t.Fatal(msg)
		}
	}

	// wrt upstream and downstream, this goroutine is "above" the main thread, so it writes to downstream and reads from
	// upstream
	go func() {
		lockWithin(t, m, "foo", 10*time.Millisecond)
		downstream <- 1
		waitFor(upstream, 2)
		downstream <- 3
		m.Unlock("foo")
	}()

	// wait until the other goroutine has locked "foo"
	waitFor(downstream, 1)

	// prove that we can lock other keys without a problem
	lockWithin(t, m, "bar", 10*time.Millisecond)
	lockWithin(t, m, "baz", 10*time.Millisecond)

	assert.Equal(t, 3, m.len(), "wrong number of internal locks")

	// start trying to lock "foo" which will block until we signal the other goroutine to unlock it
	time.AfterFunc(20*time.Millisecond, func() { upstream <- 2 })
	lockWithin(t, m, "foo", 200*time.Millisecond)

	// there better be a 3 already queued up in the downstream, otherwise we locked too fast
	readSignalNow(downstream, 3, "locked \"foo\" before the other goroutine unlocked it...")

	assert.Equal(t, 3, m.len(), "wrong number of internal locks")

	// we can unlock out of order, and locks go away as we unlock them
	m.Unlock("bar")
	assert.Equal(t, 2, m.len(), "wrong number of internal locks")

	m.Unlock("baz")
	assert.Equal(t, 1, m.len(), "wrong number of internal locks")

	m.Unlock("foo")
	assert.Equal(t, 0, m.len(), "wrong number of internal locks")
}

func TestTransientLockMapBadUnlock(t *testing.T) {
	// call a function, and fail the test if the function doesn't panic
	index := 0
	shouldPanic := func(f func()) {
		index++
		defer func() {
			if recovered := recover(); recovered != nil {
				// ok, all is well
			} else {
				// we were supposed to panic but didn't - fail the test!
				t.Fatalf("test #%d did not panic as expected...", index)
			}
		}()
		f()
	}

	// unlocking a key that has never been referenced
	m := NewTransientLockMap()
	shouldPanic(func() {
		m.Unlock("foo")
	})
	assert.Equal(t, 0, m.len(), "wrong number of internal locks")

	// double-unlocking a key in the same goroutine
	m = NewTransientLockMap()
	shouldPanic(func() {
		assertLock(t, m, "foo")
		m.Unlock("foo")
		m.Unlock("foo")
	})
	assert.Equal(t, 0, m.len(), "wrong number of internal locks")

	// double-unlocking a key across 2 goroutines
	m = NewTransientLockMap()
	shouldPanic(func() {
		signal := make(chan struct{})

		go func() {
			assertLock(t, m, "foo")
			m.Unlock("foo")
			close(signal)
		}()

		<-signal
		m.Unlock("foo")
	})
	assert.Equal(t, 0, m.len(), "wrong number of internal locks")
}

func TestTransientLockMapSequence(t *testing.T) {
	m := NewTransientLockMap()

	// signals
	partnerAboutToLock := make(chan struct{})
	partnerGotLock := make(chan struct{})

	assertLock(t, m, "foo")
	go func() {
		close(partnerAboutToLock)
		assertLock(t, m, "foo")
		close(partnerGotLock)
		time.Sleep(125 * time.Millisecond)
		m.Unlock("foo")
	}()

	<-partnerAboutToLock
	time.Sleep(100 * time.Millisecond) // give our partner time to actually call Lock()
	m.Unlock("foo")

	<-partnerGotLock
	start := time.Now()
	assertLock(t, m, "foo")

	// ensure that the prior Lock() call actually blocked and waited for a while, as intended
	if d := time.Since(start); d < 100*time.Millisecond {
		t.Fatalf("Lock acquired too fast (%s)", d)
	}
	m.Unlock("foo")
	assert.Equal(t, 0, m.len(), "wrong number of internal locks")
}

func TestTransientLockMapContention(t *testing.T) {
	m := NewTransientLockMap()

	var wg sync.WaitGroup

	assertLock(t, m, "foo")
	assertLock(t, m, "bar")

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assertLock(t, m, "foo")
			m.Unlock("foo")
			assertLock(t, m, "bar")
			m.Unlock("bar")
		}()
	}

	time.Sleep(30 * time.Millisecond)
	m.Unlock("bar")
	m.Unlock("foo")

	wg.Wait()

	assert.Equal(t, 0, m.len(), "wrong number of internal locks")
}

func TestTransientLockMapTimeout(t *testing.T) {
	m := NewTransientLockMap()

	assertLock(t, m, "foo")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res := m.Lock(ctx, "foo")
	assert.Equal(t, false, res, "lock foo (2)")

	m.Unlock("foo")
	lockWithin(t, m, "foo", 10*time.Millisecond) // should be able to lock near instantly

	ctx2, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		res := m.Lock(ctx2, "foo")
		assert.Equal(t, false, res, "lock foo (4)")
		close(done)
	}()

	time.Sleep(25 * time.Millisecond)

	// goroutine should not be done yet (i.e. Lock() should still be blocked)
	select {
	case <-done:
		t.Fatal("done should not be closed yet!")
	default:
	}

	cancel()

	select {
	case <-time.After(25 * time.Millisecond):
		t.Fatal("timeout waiting for done channel to close")
	case <-done:
		// ok
	}

	// finally, verify that we can unlock and lock still
	m.Unlock("foo")
	lockWithin(t, m, "foo", 10*time.Millisecond) // should be able to lock near instantly
	m.Unlock("foo")

	assert.Equal(t, 0, m.len(), "wrong number of internal locks")

	// We should not be able to lock with an already-cancelled context.
	for i := 0; i < 10; i++ {
		assert.Assert(t, !m.Lock(ctx2, "foo"), "lock foo (final) expected canceled")
	}
}

func TestTransientLockMapRun(t *testing.T) {
	m := NewTransientLockMap()

	start := make(chan struct{})
	var wgSucc sync.WaitGroup
	var wgFail sync.WaitGroup
	var nFail int32
	var nSucc int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 10; i++ {
		if i == 0 {
			wgSucc.Add(1)
		} else {
			wgFail.Add(1)
		}
		go func() {
			<-start
			err := m.Run(ctx, "foo", func(ctx context.Context) error {
				// Whoever got the lock should cancel everyone else.
				cancel()
				wgFail.Wait()
				return nil
			})
			if err == nil {
				defer wgSucc.Done()
				atomic.AddInt32(&nSucc, 1)
			} else {
				defer wgFail.Done()
				if err == context.Canceled {
					atomic.AddInt32(&nFail, 1)
				} else {
					t.Errorf("expected canceled, got %T: %s", err, err)
				}
			}
		}()
	}

	close(start)
	wgSucc.Wait()

	assert.Equal(t, int32(1), nSucc, "wrong # success")
	assert.Equal(t, int32(9), nFail, "wrong # failures")
}

func assertLock(t *testing.T, m *TransientLockMap, key string) {
	assert.Assert(t, m.Lock(context.Background(), key), "should have locked")
}

// grabs a lock, panicking if this takes longer than expected
func lockWithin(t *testing.T, m *TransientLockMap, key string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	assert.Assert(t, m.Lock(ctx, key), "should have locked")
}
