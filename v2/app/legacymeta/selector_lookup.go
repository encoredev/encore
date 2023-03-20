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
	services  map[selector.Selector]map[*app.Service]struct{}
	endpoints map[selector.Selector]map[*api.Endpoint]struct{}
}

// computeSelectorLookup creates a selector lookup from this application
func computeSelectorLookup(appDesc *app.Desc) *selectorLookup {
	s := &selectorLookup{
		services:  make(map[selector.Selector]map[*app.Service]struct{}),
		endpoints: make(map[selector.Selector]map[*api.Endpoint]struct{}),
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
		for _, tag := range ep.Tags {
			s.recordEndpoint(tag, ep, svc)
		}
	}

	return s
}

// recordRPC tracks both the RPC for the selector, but also the service the RPC is in
func (sm *selectorLookup) recordEndpoint(s selector.Selector, ep *api.Endpoint, svc *app.Service) {
	if sm.endpoints[s] == nil {
		sm.endpoints[s] = make(map[*api.Endpoint]struct{})
	}
	sm.endpoints[s][ep] = struct{}{}

	if sm.services[s] == nil {
		sm.services[s] = make(map[*app.Service]struct{})
	}
	sm.services[s][svc] = struct{}{}
}

// GetEndpoints returns all the rpcs which match any of the given selectors
func (sm *selectorLookup) GetEndpoints(targets selector.Set) (rtn []*api.Endpoint) {
	for _, s := range targets {
		if rpcs, found := sm.endpoints[s]; found {
			for rpc := range rpcs {
				rtn = append(rtn, rpc)
			}
		}
	}
	return
}

// GetServices returns all services which match any of the given selectors
func (sm *selectorLookup) GetServices(targets selector.Set) (rtn []*app.Service) {
	for _, s := range targets {
		if services, found := sm.services[s]; found {
			for svc := range services {
				rtn = append(rtn, svc)
			}
		}
	}
	return
}
