package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/servicestruct"
)

func configureAPIFramework(pc *parsectx.Context, services []*Service, apiPackages []*apis.ParseResult) option.Option[*apiframework.AppDesc] {
	if len(apiPackages) == 0 {
		return option.None[*apiframework.AppDesc]()
	}

	fw := &apiframework.AppDesc{}

	for _, pkg := range apiPackages {
		// First handle global API framework usage
		// i.e. auth handlers and middleware which apply across all services

		// Add the middleware
		var svcMiddleware []*middleware.Middleware
		for _, mw := range pkg.Middleware {
			if mw.Global {
				fw.GlobalMiddleware = append(fw.GlobalMiddleware, mw)
			} else {
				svcMiddleware = append(svcMiddleware, mw)
			}
		}

		// Add the app's auth handler
		for _, ah := range pkg.AuthHandlers {
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

		// Handle service specific API framework usage
		if len(pkg.Endpoints) > 0 || len(pkg.ServiceStructs) > 0 || len(svcMiddleware) > 0 {
			// Find the service that this API package belongs to
			var service *Service
			var svcIdx int
			for i, svc := range services {
				if pkg.Pkg.FSPath.HasPrefix(svc.FSRoot) {
					service = svc
					svcIdx = i
					break
				}
			}
			if service == nil {
				pc.Errs.Add(errNoServiceFound(pkg.Pkg.Name))
				continue
			}

			svcDesc := service.Framework.GetOrElse(&apiframework.ServiceDesc{
				Num:     svcIdx,
				RootPkg: pkg.Pkg,
			})

			svcDesc.Middleware = append(svcDesc.Middleware, svcMiddleware...)
			svcDesc.Endpoints = append(svcDesc.Endpoints, pkg.Endpoints...)

			// We only allow one service struct per service, however the parser might have found multiple
			for _, serviceStruct := range pkg.ServiceStructs {
				if svcDesc.ServiceStruct.Empty() {
					svcDesc.ServiceStruct = option.Some(serviceStruct)
				} else {
					pc.Errs.Add(
						servicestruct.ErrDuplicateServiceStructs.
							AtGoNode(svcDesc.ServiceStruct.MustGet().Decl.AST, errors.AsError("first service struct defined here")).
							AtGoNode(serviceStruct.Decl.AST, errors.AsError("second service struct defined here")),
					)
				}
			}

			service.Framework = option.Some(svcDesc)
		}
	}

	return option.Some(fw)
}
