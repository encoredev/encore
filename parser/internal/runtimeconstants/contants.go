package runtimeconstants

import (
	"time"
)

var constants = map[string]map[string]int64{
	"encore.dev/pubsub": {
		"NoRetries":       -2,
		"InfiniteRetries": -1,
		"AtLeastOnce":     1,
	},
	"encore.dev/cron": {
		"Minute": 60,
		"Hour":   60 * 60,
	},
	"time": {
		"Nanosecond":  int64(time.Nanosecond),
		"Microsecond": int64(time.Microsecond),
		"Millisecond": int64(time.Millisecond),
		"Second":      int64(time.Second),
		"Minute":      int64(time.Minute),
		"Hour":        int64(time.Hour),
	},
}

// Get returns the value of a constant within the runtime, if it's registered in this package
func Get(pkg, ident string) (int64, bool) {
	pkgMap, found := constants[pkg]
	if !found {
		return 0, false
	}

	value, found := pkgMap[ident]
	return value, found
}
