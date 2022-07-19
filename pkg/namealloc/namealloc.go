package namealloc

import (
	"go/token"
	"strconv"
)

// Allocator helps choosing names without collisions
// and without using Go keywords. The zero value is ready
// to be used.
type Allocator struct {
	// Reserved decides whether a given input is reserved.
	// If nil it defaults to token.IsKeyword.
	Reserved func(input string) bool

	used map[string]bool
}

// Get allocates a name that is a valid, unused identifier
// based on the input string. It ensures the same name is not
// returned multiple times even for the same input.
func (a *Allocator) Get(input string) (name string) {
	if a.isReserved(input) {
		input = input + "_"
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

func (a *Allocator) isReserved(input string) bool {
	reserved := a.Reserved
	if reserved == nil {
		reserved = token.IsKeyword
	}
	return reserved(input)
}
