//go:build encore_app

package trace2

import (
	"sync"
	_ "unsafe" // for go:linkname
)

// mutex wraps sync.Mutex for compatibility
type mutex struct {
	sync.Mutex
}

func mutexLock(mut *mutex) {
	mut.Lock()
}

func mutexUnlock(mut *mutex) {
	mut.Unlock()
}
