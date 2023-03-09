package app

import (
	"fmt"

	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/option"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/apis/middleware"
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
				pc.Errs.AddSrcErr(
					srcerrors.MultipleAuthHandlersFound(
						pc.Errs.FS(),
						fw.AuthHandler.MustGet().Decl.AST.Type, ah.Decl.AST.Type,
					),
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
				pc.Errs.AddStd(
					srcerrors.StandardLibraryError(fmt.Errorf("service not found for API framework package %s", pkg.Pkg.Name)),
				)
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
					pc.Errs.AddSrcErr(
						srcerrors.MultipleServiceStructsFound(
							pc.Errs.FS(),
							service.Name,
							svcDesc.ServiceStruct.MustGet().Decl.AST, serviceStruct.Decl.AST,
						),
					)
				}
			}

			service.Framework = option.Some(svcDesc)
		}
	}

	return option.Some(fw)
}
