package noopgwdesc

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"encr.dev/pkg/noopgateway"
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// Describe computes a Description based on the given metadata and service discovery configuration.
// If serviceDiscovery is nil, the routes will be added but the service discovery setup will be empty.
func Describe(md *meta.Data, serviceDiscovery map[noopgateway.ServiceName]string) *noopgateway.Description {
	desc := &noopgateway.Description{
		Services: make(map[noopgateway.ServiceName]noopgateway.Service),
	}

	for _, svc := range md.Svcs {
		svcName := noopgateway.ServiceName(svc.Name)

		if serviceDiscovery != nil {
			host, ok := serviceDiscovery[svcName]
			if !ok {
				continue
			}

			target := &url.URL{
				Scheme: "http",
				Host:   host,
			}
			desc.Services[svcName] = noopgateway.Service{
				URL: target,
			}
		}

		for _, ep := range svc.Rpcs {
			methods := ep.HttpMethods
			if slices.Contains(methods, "*") {
				methods = []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
			}
			desc.Routes = append(desc.Routes, &noopgateway.Route{
				Methods:      methods,
				Dest:         svcName,
				RequiresAuth: ep.AccessType == meta.RPC_AUTH,
				Path:         pathToString(ep.Path),
			})
		}
	}

	return desc
}

func pathToString(path *meta.Path) string {
	parts := make([]string, 0, len(path.Segments))
	paramIdx := 0
	for _, seg := range path.Segments {
		var val string
		switch seg.Type {
		case meta.PathSegment_LITERAL:
			val = seg.Value
		case meta.PathSegment_PARAM:
			val = fmt.Sprintf(":p%d", paramIdx)
			paramIdx++
		case meta.PathSegment_WILDCARD, meta.PathSegment_FALLBACK:
			val = fmt.Sprintf("*p%d", paramIdx)
			paramIdx++
		}
		parts = append(parts, val)
	}
	return "/" + strings.Join(parts, "/")
}
