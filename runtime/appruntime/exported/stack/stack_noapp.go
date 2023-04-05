//go:build !encore_app

package stack

import "runtime"

func encoreCallers(skip int, pc []uintptr) (n int, off uintptr) {
	n = runtime.Callers(skip, pc)

	// Outside of an Encore app we can't easily determine the offset.
	// This is only relevant to be able to construct ASLR-independent
	// program counters for doing PC lookups from other processes,
	// such as during trace rendering. This is not something we need
	// to worry about when not running inside an Encore app.
	off = 0

	return
}
