package app

import (
	"sort"

	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/selector"
	"encr.dev/v2/parser/infra"
)

// Desc describes an Encore application.
type Desc struct {
	Errs *perr.List

	Services []*Service
	Infra    *infra.Desc

	// Packages are the packages that are contained within
	// the application. It does not list packages that have been
	// parsed but belong to dependencies.
	Packages []*pkginfo.Package

	// Framework describes API Framework-specific application-global data.
	Framework option.Option[*apiframework.AppDesc]
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

// Service describes an Encore service.
type Service struct {
	// Name is the name of the service.
	Name string

	// FSRoot is the root directory of the service.
	FSRoot paths.FS

	// Framework contains API Framework-specific data for this service.
	Framework option.Option[*apiframework.ServiceDesc]

	// InfraUsage describes the infra resources the service accesses and how.
	InfraUsage []any // type TBD
}

// ValidateAndDescribe validates the application and computes the
// application description.
func ValidateAndDescribe(pc *parsectx.Context, result parser.Result) *Desc {
	defer pc.Trace("app.ValidateAndDescribe").Done()

	// First we want to discover the service layout
	services := discoverServices(pc, result)

	// Now we can configure the API framework by combining the service information
	// with the parse results.
	framework := configureAPIFramework(pc, services, result.APIs)

	desc := &Desc{
		Errs:      pc.Errs,
		Packages:  result.Packages,
		Services:  services,
		Framework: framework,
		Infra:     result.Infra,
	}

	// Run the application-level validations against the application description.
	desc.validate(pc)

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
