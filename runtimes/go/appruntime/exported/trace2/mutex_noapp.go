//go:build !encore_app

package trace2

import "sync"

type mutex struct {
	mut sync.Mutex
}

func mutexLock(m *mutex) { m.mut.Lock() }

func mutexUnlock(m *mutex) { m.mut.Unlock() }
