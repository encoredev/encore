//go:build encore_app

package trace

import _ "unsafe" // for go:linkname

// mutex must exactly match implementation in the runtime.
type mutex struct {
	key uintptr
}

//go:linkname mutexLock runtime.lock
func mutexLock(mut *mutex)

//go:linkname mutexUnlock runtime.unlock
func mutexUnlock(mut *mutex)
