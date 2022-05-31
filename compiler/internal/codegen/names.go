package codegen

import (
	"go/token"
	"strconv"
)

// nameAllocator helps choosing names without collisions
// and without using Go keywords. The zero value is ready
// to be used.
type nameAllocator struct {
	used map[string]bool
}

// Get allocates a name that is a valid, unused identifier
// based on the input string. It ensures the same name is not
// returned multiple times even for the same input.
func (a *nameAllocator) Get(input string) (name string) {
	if token.IsKeyword(input) {
		input = "_" + input
	}

	candidate := input
	for i := 2; a.used[candidate]; i++ {
		candidate = input + strconv.Itoa(i)
	}

	if a.used == nil {
		a.used = make(map[string]bool)
	}
	a.used[candidate] = true
	return candidate
}
