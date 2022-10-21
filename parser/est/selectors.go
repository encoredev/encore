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

// SelectorLookup is a helper cache for looking up services and RPC's by selector
type SelectorLookup struct {
	services map[selector.Selector]map[*Service]struct{}
	rpcs     map[selector.Selector]map[*RPC]struct{}
}

// SelectorLookup creates a selector lookup from this application
func (a *Application) SelectorLookup() *SelectorLookup {
	s := &SelectorLookup{
		services: make(map[selector.Selector]map[*Service]struct{}),
		rpcs:     make(map[selector.Selector]map[*RPC]struct{}),
	}

	// Record all RPC's
	for _, svc := range a.Services {
		for _, rpc := range svc.RPCs {
			// Track against all
			s.recordRPC(selector.Selector{Type: selector.All}, rpc)

			// Track against any defined tags
			for _, tag := range rpc.Tags {
				s.recordRPC(tag, rpc)
			}
		}
	}

	return s
}

// recordRPC tracks both the RPC for the selector, but also the service the RPC is in
func (sm *SelectorLookup) recordRPC(s selector.Selector, rpc *RPC) {
	if sm.rpcs[s] == nil {
		sm.rpcs[s] = make(map[*RPC]struct{})
	}
	sm.rpcs[s][rpc] = struct{}{}

	if sm.services[s] == nil {
		sm.services[s] = make(map[*Service]struct{})
	}
	sm.services[s][rpc.Svc] = struct{}{}
}

// GetRPCs returns all the rpcs which match any of the given selectors
func (sm *SelectorLookup) GetRPCs(targets selector.Set) (rtn []*RPC) {
	for _, s := range targets {
		if rpcs, found := sm.rpcs[s]; found {
			for rpc := range rpcs {
				rtn = append(rtn, rpc)
			}
		}
	}
	return
}

// GetServices returns all services which match any of the given selectors
func (sm *SelectorLookup) GetServices(targets selector.Set) (rtn []*Service) {
	for _, s := range targets {
		if services, found := sm.services[s]; found {
			for svc := range services {
				rtn = append(rtn, svc)
			}
		}
	}
	return
}
