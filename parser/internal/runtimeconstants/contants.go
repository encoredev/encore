package runtimeconstants

var constants = map[string]map[string]any{
	"encore.dev/pubsub": {
		"NoRetries":       -2,
		"InfiniteRetries": -1,
		"AtLeastOnce":     1,
	},
}

// Get returns the value of a constant within the runtime, if it's registered in this package
func Get(pkg, ident string) (any, bool) {
	pkgMap, found := constants[pkg]
	if !found {
		return nil, false
	}

	value, found := pkgMap[ident]
	return value, found
}
