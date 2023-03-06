package app

import (
	"encr.dev/pkg/option"
	meta "encr.dev/proto/encore/parser/meta/v1"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/resource"
)

// Desc describes an Encore application.
type Desc struct {
	Services       []*Service
	InfraResources []resource.Resource
	LegacyMeta     *meta.Data

	// Framework describes API Framework-specific application-global data.
	Framework option.Option[*apiframework.AppDesc]
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
func ValidateAndDescribe(pc *parsectx.Context, result parser.Result) {

}

/*
// AddResult adds a parse result for a specific package.
func (b *Builder) AddResult(res *apis.ParseResult) {
	b.results = append(b.results, res)
}

// Build computes the application description.
func (b *Builder) Build() *AppDesc {
	d := &AppDesc{
		errs: b.errs,
	}

	setAuthHandler := func(ah *authhandler.AuthHandler) {
		if d.AuthHandler.IsPresent() {
			b.errs.Addf(ah.Decl.AST.Pos(), "multiple auth handlers defined (previous definition at %s)",
				b.pc.FS.Position(d.AuthHandler.MustGet().Decl.AST.Pos()))
		} else {
			d.AuthHandler = option.Some(ah)
		}
	}

	for _, res := range b.results {
		d.Middleware = append(d.Middleware, res.Middleware...)
		for _, ah := range res.AuthHandlers {
			setAuthHandler(ah)
		}
		if len(res.Endpoints) > 0 {
			// TODO(andre) This does not handle service creation
			// from a Pub/Sub subscription (or service struct definition).
			svc := &Service{
				Service: &service.Service{
					Name:   res.Pkg.Name,
					FSRoot: res.Pkg.FSPath,
				},
				RootPkg:   res.Pkg,
				Endpoints: res.Endpoints,
			}
			d.Services = append(d.Services, svc)
		}
	}

	// Sort the services by import path so it's easier to find
	// a service by package path, to make the output deterministic,
	// and to aid validation.
	slices.SortFunc(d.Services, func(a, b *Service) bool {
		return a.RootPkg.ImportPath < b.RootPkg.ImportPath
	})

	// Validate services are not children of one another
	for i, svc := range d.Services {
		// If the service is in a subdirectory of another service that's an error.
		for j := i - 1; j >= 0; j-- {
			thisPath := svc.RootPkg.ImportPath.String()
			otherSvc := d.Services[j]
			otherPath := otherSvc.RootPkg.ImportPath.String()
			if strings.HasPrefix(thisPath, otherPath+"/") {
				b.errs.Addf(svc.RootPkg.AST.Pos(), "service %q cannot be in a subdirectory of service %q",
					svc.Name, otherSvc.Name)
			}
		}
	}

	return d
}

// ServiceForPkg returns the service a given package belongs to, if any.
func (d *AppDesc) ServiceForPkg(path paths.Pkg) (*Service, bool) {
	idx := sort.Search(len(d.Services), func(i int) bool {
		return d.Services[i].RootPkg.ImportPath >= path
	})

	// Do we have an exact match?
	if idx < len(d.Services) && d.Services[idx].RootPkg.ImportPath == path {
		return d.Services[idx], true
	}

	// Is this package contained within the preceding service?
	if idx > 0 {
		prev := d.Services[idx-1]
		prevPath := prev.RootPkg.ImportPath.String()
		if strings.HasPrefix(path.String(), prevPath+"/") {
			return prev, true
		}
	}

	return nil, false
}

*/
