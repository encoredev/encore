package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
)

func configureAPIFramework(pc *parsectx.Context, services []*Service, res *parser.Result) option.Option[*apiframework.AppDesc] {
	var (
		endpoints      = parser.Resources[*api.Endpoint](res)
		middlewares    = parser.Resources[*middleware.Middleware](res)
		authHandlers   = parser.Resources[*authhandler.AuthHandler](res)
		serviceStructs = parser.Resources[*servicestruct.ServiceStruct](res)
	)

	if len(endpoints) == 0 && len(middlewares) == 0 && len(authHandlers) == 0 && len(serviceStructs) == 0 {
		return option.None[*apiframework.AppDesc]()
	}

	fw := &apiframework.AppDesc{}

	// First handle global API framework usage
	// i.e. auth handlers and middleware which apply across all services

	// Add the middleware
	var svcMiddleware []*middleware.Middleware
	for _, mw := range middlewares {
		if mw.Global {
			fw.GlobalMiddleware = append(fw.GlobalMiddleware, mw)
		} else {
			svcMiddleware = append(svcMiddleware, mw)
		}
	}

	// Add the app's auth handler
	for _, ah := range authHandlers {
		if fw.AuthHandler.Empty() {
			fw.AuthHandler = option.Some(ah)
		} else {
			pc.Errs.Add(
				authhandler.ErrMultipleAuthHandlers.
					AtGoNode(fw.AuthHandler.MustGet().Decl.AST.Type, errors.AsError("first auth handler defined here")).
					AtGoNode(ah.Decl.AST.Type, errors.AsError("second auth handler defined here")),
			)
		}
	}

	modifySvcDesc := func(pkg *pkginfo.Package, errTemplate *errors.Template, fn func(svc *Service, desc *apiframework.ServiceDesc)) {
		for i, svc := range services {
			if pkg.FSPath.HasPrefix(svc.FSRoot) {
				// We've found the service. Initialize the framework service description
				// if necessary, and then call fn.
				desc, ok := svc.Framework.Get()
				if !ok {
					desc = &apiframework.ServiceDesc{
						Num:     i,
						RootPkg: pkg,
					}
					svc.Framework = option.Some(desc)
				}
				fn(svc, desc)
				return
			}
		}

		// We couldn't find the service. Add an error and don't call fn.
		if errTemplate != nil {
			pc.Errs.Add(*errTemplate)
		} else {
			pc.Errs.Add(errNoServiceFound(pkg.ImportPath))
		}
	}

	for _, ep := range endpoints {
		modifySvcDesc(ep.Package(), nil, func(svc *Service, desc *apiframework.ServiceDesc) {
			desc.Endpoints = append(desc.Endpoints, ep)
		})
	}

	for _, mw := range middlewares {
		if !mw.Global {
			missingErr := middleware.ErrSvcMiddlewareNotInService.AtGoNode(mw.Decl.AST.Name)

			// Per-service middleware.
			modifySvcDesc(mw.Package(), &missingErr, func(svc *Service, desc *apiframework.ServiceDesc) {
				desc.Middleware = append(desc.Middleware, mw)
			})
		}
	}

	for _, ss := range serviceStructs {
		modifySvcDesc(ss.Package(), nil, func(svc *Service, desc *apiframework.ServiceDesc) {
			if desc.ServiceStruct.Empty() {
				desc.ServiceStruct = option.Some(ss)
			} else {
				pc.Errs.Add(
					servicestruct.ErrDuplicateServiceStructs.
						AtGoNode(desc.ServiceStruct.MustGet().Decl.AST, errors.AsError("first service struct defined here")).
						AtGoNode(ss.Decl.AST, errors.AsError("second service struct defined here")),
				)
			}
		})
	}

	return option.Some(fw)
}
