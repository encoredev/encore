package apiframework

import (
	"sort"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/service"
)

// AppDesc describes an Encore Framework-based application.
type AppDesc struct {
	errs *perr.List

	Services   []*Service
	Middleware []*middleware.Middleware

	// AuthHandler defines the application's auth handler, if any.
	AuthHandler option.Option[*authhandler.AuthHandler]
}

// Service is an Encore Framework-based service.
//
// For code that deals with general services, use *service.Service instead of this type.
type Service struct {
	*service.Service

	// Num is the service number in the application.
	Num int

	// RootPkg is the root package of the service.
	RootPkg *pkginfo.Package

	// Endpoints are the endpoints defined in this service.
	Endpoints []*api.Endpoint

	// ServiceStruct defines the service's service struct, if any.
	ServiceStruct option.Option[*servicestruct.ServiceStruct]
}

// NewBuilder creates a new Builder.
func NewBuilder(pc *parsectx.Context) *Builder {
	return &Builder{pc: pc, errs: pc.Errs}
}

// Builder is used to build an application description.
type Builder struct {
	pc      *parsectx.Context
	errs    *perr.List
	results []*apis.ParseResult
}

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
