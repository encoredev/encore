package est

import (
	"encr.dev/parser/selector"
)

// MatchingMiddleware reports which middleware applies to the given RPC,
// and the order they apply in.
func (a *Application) MatchingMiddleware(rpc *RPC) []*Middleware {
	tags := make(map[string]bool, len(rpc.Tags))
	for _, tag := range rpc.Tags {
		tags[tag.Value] = true
	}

	match := func(s selector.Selector) bool {
		switch s.Type {
		case selector.Tag:
			return tags[s.Value]
		case selector.All:
			return true
		default:
			return false
		}
	}

	var matches []*Middleware

	// Ensure middleware ordering is preserved.

	// First add global middleware.
	for _, mw := range a.Middleware {
		if mw.Global {
			for _, s := range mw.Target {
				if match(s) {
					matches = append(matches, mw)
				}
			}
		}
	}

	// Then add service-specific middleware.
	for _, mw := range rpc.Svc.Middleware {
		for _, s := range mw.Target {
			if match(s) {
				matches = append(matches, mw)
			}
		}
	}
	return matches
}
