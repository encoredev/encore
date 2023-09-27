//go:build encore_app

package stack

import _ "unsafe" // for go:linkname

//go:linkname encoreCallers runtime.encoreCallers
func encoreCallers(skip int, pc []uintptr) (n int, off uintptr)
