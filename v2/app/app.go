package app

import (
	"sort"

	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/selector"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/usage"
)

// Desc describes an Encore application.
type Desc struct {
	Errs *perr.List

	Parse    *parser.Result
	Services []*Service

	// Framework describes API Framework-specific application-global data.
	Framework option.Option[*apiframework.AppDesc]

	// ResourceUsageOutsideServices describes resources that are used outside of a service.
	ResourceUsageOutsideServices map[resource.Resource][]usage.Usage
}

// MatchingMiddleware reports which middleware applies to the given RPC,
// and the order they apply in.
func (d *Desc) MatchingMiddleware(ep *api.Endpoint) []*middleware.Middleware {
	tags := make(map[string]bool, len(ep.Tags))
	for _, tag := range ep.Tags {
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

	var matches []*middleware.Middleware

	// Ensure middleware ordering is preserved.

	// First add global middleware.
	d.Framework.ForAll(func(fw *apiframework.AppDesc) {
		for _, mw := range fw.GlobalMiddleware {
			if mw.Global {
				for _, s := range mw.Target {
					if match(s) {
						matches = append(matches, mw)
					}
				}
			}
		}
	})

	// Then add service-specific middleware.
	if svc, ok := d.ServiceForPath(ep.File.Pkg.FSPath); ok {
		svc.Framework.ForAll(func(fw *apiframework.ServiceDesc) {
			for _, mw := range fw.Middleware {
				for _, s := range mw.Target {
					if match(s) {
						matches = append(matches, mw)
					}
				}
			}
		})
	}

	return matches
}

// ValidateAndDescribe validates the application and computes the
// application description.
func ValidateAndDescribe(pc *parsectx.Context, result *parser.Result) *Desc {
	defer pc.Trace("app.ValidateAndDescribe").Done()

	// First we want to discover the service layout
	services := discoverServices(pc, result)

	// Now we can configure the API framework by combining the service information
	// with the parse results.
	framework := configureAPIFramework(pc, services, result)

	desc := &Desc{
		Errs:                         pc.Errs,
		Parse:                        result,
		Services:                     services,
		Framework:                    framework,
		ResourceUsageOutsideServices: make(map[resource.Resource][]usage.Usage),
	}

	// Find each services infra usage.
	desc.locateResourceUsage(result)

	// Run the application-level validations against the application description.
	desc.validate(pc, result)

	return desc
}

// ServiceForPath returns the service a given folder path belongs to, if any.
func (d *Desc) ServiceForPath(path paths.FS) (*Service, bool) {
	idx := sort.Search(len(d.Services), func(i int) bool {
		return d.Services[i].FSRoot.ToIO() > path.ToIO()
	})

	// Is the path contained within the service at idx?
	if idx < len(d.Services) && path.HasPrefix(d.Services[idx].FSRoot) {
		return d.Services[idx], true
	}

	// Is this path contained within the preceding service?
	if idx > 0 {
		prev := d.Services[idx-1]
		if path.HasPrefix(prev.FSRoot) {
			return prev, true
		}
	}

	return nil, false
}
