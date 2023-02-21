package literals

import (
	"go/constant"
	"time"

	"encore.dev/storage/cache"
	"encr.dev/parser2/internal/paths"
)

var constants = map[paths.Pkg]map[string]any{
	"encore.dev/pubsub": {
		"NoRetries":       -2,
		"InfiniteRetries": -1,
		"AtLeastOnce":     1,
	},
	"encore.dev/cron": {
		"Minute": 60,
		"Hour":   60 * 60,
	},
	"encore.dev/storage/cache": {
		"AllKeysLRU":     string(cache.AllKeysLRU),
		"AllKeysLFU":     string(cache.AllKeysLFU),
		"AllKeysRandom":  string(cache.AllKeysRandom),
		"VolatileLRU":    string(cache.VolatileLRU),
		"VolatileLFU":    string(cache.VolatileLFU),
		"VolatileTTL":    string(cache.VolatileTTL),
		"VolatileRandom": string(cache.VolatileRandom),
		"NoEviction":     string(cache.NoEviction),
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

// runtimeConstant returns the value of a constant within the runtime,
// if it's known to this file.
func runtimeConstant(pkg paths.Pkg, name string) (constant.Value, bool) {
	pkgMap, found := constants[pkg]
	if !found {
		return constant.MakeUnknown(), false
	}

	if value, found := pkgMap[name]; found {
		// constant.Make recognizes int64 but not int.
		// If we have an int, turn it to int64.
		if val, ok := value.(int); ok {
			return constant.Make(int64(val)), true
		}
		return constant.Make(value), true
	}
	return constant.MakeUnknown(), false
}
