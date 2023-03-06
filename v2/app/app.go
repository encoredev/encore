package app

import (
	"sort"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/infra/resource"
)

// Desc describes an Encore application.
type Desc struct {
	errs *perr.List

	Services       []*Service
	InfraResources []resource.Resource

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
func ValidateAndDescribe(pc *parsectx.Context, result parser.Result) *Desc {
	d := &Desc{
		errs: pc.Errs,
	}

	fw := &apiframework.AppDesc{}
	d.Framework = option.Some(fw)

	setAuthHandler := func(ah *authhandler.AuthHandler) {
		if fw.AuthHandler.IsPresent() {
			pc.Errs.Addf(ah.Decl.AST.Pos(), "multiple auth handlers defined (previous definition at %s)",
				pc.FS.Position(fw.AuthHandler.MustGet().Decl.AST.Pos()))
		} else {
			fw.AuthHandler = option.Some(ah)
		}
	}

	for _, res := range result.APIs {
		var svcMiddleware []*middleware.Middleware
		for _, mw := range res.Middleware {
			if mw.Global {
				fw.GlobalMiddleware = append(fw.GlobalMiddleware, mw)
			} else {
				// TODO(andre) Validate that the middleware is within a service.
				svcMiddleware = append(svcMiddleware, mw)
			}
		}

		for _, ah := range res.AuthHandlers {
			setAuthHandler(ah)
		}
		if len(res.Endpoints) > 0 {
			// TODO(andre) This does not handle service creation
			// from a Pub/Sub subscription (or service struct definition).
			svc := &Service{
				Name:   res.Pkg.Name,
				FSRoot: res.Pkg.FSPath,
				Framework: option.Some(&apiframework.ServiceDesc{
					RootPkg:    res.Pkg,
					Endpoints:  res.Endpoints,
					Middleware: svcMiddleware,
				}),
			}
			d.Services = append(d.Services, svc)
		}
	}

	// Sort the services by import path so it's easier to find
	// a service by package path, to make the output deterministic,
	// and to aid validation.
	slices.SortFunc(d.Services, func(a, b *Service) bool {
		return a.Framework.MustGet().RootPkg.ImportPath < b.Framework.MustGet().RootPkg.ImportPath
	})

	// Validate services are not children of one another
	for i, svc := range d.Services {
		thisSvc := svc.Framework.MustGet()
		// If the service is in a subdirectory of another service that's an error.
		for j := i - 1; j >= 0; j-- {
			thisPath := thisSvc.RootPkg.ImportPath.String()
			otherSvc := d.Services[j].Framework.MustGet()
			otherPath := otherSvc.RootPkg.ImportPath.String()
			if strings.HasPrefix(thisPath, otherPath+"/") {
				pc.Errs.Addf(thisSvc.RootPkg.AST.Pos(), "service %q cannot be in a subdirectory of service %q",
					svc.Name, d.Services[j].Name)
			}
		}
	}

	return d
}

// FrameworkServiceForPkg returns the service a given package belongs to, if any.
// It only considers framework services.
func (d *Desc) FrameworkServiceForPkg(path paths.Pkg) (*Service, bool) {
	idx := sort.Search(len(d.Services), func(i int) bool {
		return d.Services[i].Framework.MustGet().RootPkg.ImportPath >= path
	})

	// Do we have an exact match?
	if idx < len(d.Services) && d.Services[idx].Framework.MustGet().RootPkg.ImportPath == path {
		return d.Services[idx], true
	}

	// Is this package contained within the preceding service?
	if idx > 0 {
		prev := d.Services[idx-1]
		prevPath := prev.Framework.MustGet().RootPkg.ImportPath.String()
		if strings.HasPrefix(path.String(), prevPath+"/") {
			return prev, true
		}
	}

	return nil, false
}
