package experiment

import (
	"os"
	"strings"
)

type Experiment string

// No experiments currently defined.

// Enabled reports whether the given experiment is enabled.
func Enabled(x Experiment) bool {
	fields := strings.Fields(os.Getenv("ENCORE_EXPERIMENT"))
	for _, f := range fields {
		if f == string(x) {
			return true
		}
	}
	return false
}
