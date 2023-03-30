package legacymeta

import (
	"fmt"

	"encr.dev/v2/app"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/selector"
)

// selectorLookup is a helper cache for looking up services and RPC's by selector
type selectorLookup struct {
	services  map[selector.Key]map[*app.Service]struct{}
	endpoints map[selector.Key]map[*api.Endpoint]struct{}
}

// computeSelectorLookup creates a selector lookup from this application
func computeSelectorLookup(appDesc *app.Desc) *selectorLookup {
	s := &selectorLookup{
		services:  make(map[selector.Key]map[*app.Service]struct{}),
		endpoints: make(map[selector.Key]map[*api.Endpoint]struct{}),
	}

	// Record all RPCs
	for _, ep := range parser.Resources[*api.Endpoint](appDesc.Parse) {
		svc, ok := appDesc.ServiceForPath(ep.File.FSPath)
		if !ok {
			panic(fmt.Sprintf("no service found for endpoint %s.%s", ep.File.Pkg.Name, ep.Name))
		}

		// Track against all
		s.recordEndpoint(selector.Selector{Type: selector.All}, ep, svc)

		// Track against any defined tags
		ep.Tags.ForEach(func(sel selector.Selector) {
			s.recordEndpoint(sel, ep, svc)
		})
	}

	return s
}

// recordRPC tracks both the RPC for the selector, but also the service the RPC is in
func (sm *selectorLookup) recordEndpoint(s selector.Selector, ep *api.Endpoint, svc *app.Service) {
	key := s.Key()
	if sm.endpoints[key] == nil {
		sm.endpoints[key] = make(map[*api.Endpoint]struct{})
	}
	sm.endpoints[key][ep] = struct{}{}

	if sm.services[key] == nil {
		sm.services[key] = make(map[*app.Service]struct{})
	}
	sm.services[key][svc] = struct{}{}
}

// GetEndpoints returns all the rpcs which match any of the given selectors
func (sm *selectorLookup) GetEndpoints(targets selector.Set) (rtn []*api.Endpoint) {
	targets.ForEach(func(s selector.Selector) {
		if rpcs, found := sm.endpoints[s.Key()]; found {
			for rpc := range rpcs {
				rtn = append(rtn, rpc)
			}
		}
	})
	return
}

// GetServices returns all services which match any of the given selectors
func (sm *selectorLookup) GetServices(targets selector.Set) (rtn []*app.Service) {
	targets.ForEach(func(s selector.Selector) {
		if services, found := sm.services[s.Key()]; found {
			for svc := range services {
				rtn = append(rtn, svc)
			}
		}
	})
	return
}
