// Package debugflags parses the ENCOREDEBUG environment variable, a
// comma-separated list of key=value pairs used to toggle non-default daemon
// behavior. Example: ENCOREDEBUG=sqldbrole=legacy,foo=bar.
package debugflags

import (
	"os"
	"strings"
	"sync"
)

// SQLDBRoleLegacy is the value of the "sqldbrole" flag that selects the
// pre-encore_services SQL role behavior.
const SQLDBRoleLegacy = "legacy"

// Get returns the value of the named debug flag, or "" if it is not set.
func Get(name string) string {
	flagsOnce.Do(func() {
		flags = parse(os.Getenv("ENCOREDEBUG"))
	})
	return flags[name]
}

var (
	flagsOnce sync.Once
	flags     map[string]string
)

func parse(s string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, _ := strings.Cut(part, "=")
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}
