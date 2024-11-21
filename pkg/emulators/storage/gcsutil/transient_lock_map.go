package gcsutil

import (
	"context"
	"fmt"
	"sync"
)

// TransientLockMap is a map of mutexes that is safe for concurrent access.  It does not bother to save mutexes after
// they have been unlocked, and thus this data structure is best for situations where the space of keys is very large.
// If the space of keys is small then it may be inefficient to constantly recreate mutexes whenever they are needed.
type TransientLockMap struct {
	mu    sync.Mutex              // the mutex that locks the map
	locks map[string]*countedLock // all locks that are currently held
}

// NewTransientLockMap returns a new TransientLockMap.
func NewTransientLockMap() *TransientLockMap {
	return &TransientLockMap{
		locks: make(map[string]*countedLock),
	}
}

// Lock acquires the lock for the specified key and returns true, unless the context finishes before the lock could be
// acquired, in which case false is returned.
func (l *TransientLockMap) Lock(ctx context.Context, key string) bool {
	lock := func() *countedLock {
		// If there is high lock contention, we could use a readonly lock to check if the lock is already in the map (and
		// thus no map writes are necessary), but this is complicated enough as it is so we skip that optimization for now.
		l.mu.Lock()
		defer l.mu.Unlock()

		// Check if there is already a lock for this key.
		lock, ok := l.locks[key]
		if !ok {
			// no lock yet, so make one and add it to the map
			lock = newCountedLock()
			l.locks[key] = lock
		}

		// Order is very important here.  First we have to increment the refcount while we still have the map locked; this
		// will prevent anyone else from evicting this lock after we unlock the map but before we lock the key.  Second we
		// have to unlock the map _before_ we start trying to lock the key (because locking the key could take a long time
		// and we don't want to keep the map locked that whole time).
		lock.refcount++ // incremented while holding _map_ lock
		return lock
	}()

	if !lock.Lock(ctx) {
		l.returnLockObj(key, lock)
		return false
	}
	return true
}

// Unlock unlocks the lock for the specified key. Panics if the lock is not currently held.
func (l *TransientLockMap) Unlock(key string) {
	lock := func() *countedLock {
		l.mu.Lock()
		defer l.mu.Unlock()

		lock, ok := l.locks[key]
		if !ok {
			panic(fmt.Sprintf("lock not held for key %s", key))
		}
		return lock
	}()

	lock.Unlock()
	l.returnLockObj(key, lock)
}

// Run runs the given callback while holding the lock, unless the context finishes before the lock could be
// acquired, in which case the context error is returned.
func (l *TransientLockMap) Run(ctx context.Context, key string, f func(ctx context.Context) error) error {
	if !l.Lock(ctx, key) {
		return ctx.Err()
	}
	defer l.Unlock(key)
	return f(ctx)
}

func (l *TransientLockMap) returnLockObj(key string, lock *countedLock) {
	l.mu.Lock()
	defer l.mu.Unlock()

	lock.refcount--
	if lock.refcount < 0 {
		panic(fmt.Sprintf("BUG: somehow the lock.refcount for %q dropped to %d", key, lock.refcount))
	}
	if lock.refcount == 0 {
		delete(l.locks, key)
	}
}
