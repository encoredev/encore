package app

import (
	"fmt"

	"encr.dev/pkg/errors"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/parser/apis/middleware"
	"encr.dev/v2/parser/apis/selector"
)

func (d *Desc) validateMiddleware(pc *parsectx.Context, fw *apiframework.AppDesc) {
	var allTags selector.Set

	for _, svc := range d.Services {
		var svcTags selector.Set

		if fwSvc, ok := svc.Framework.Get(); ok {
			// Collect all tags used in the service.
			for _, ep := range fwSvc.Endpoints {
				svcTags.Merge(ep.Tags)
			}

			// Check that service middleware targets are valid.
			for _, m := range fwSvc.Middleware {
				m.Target.ForEach(func(s selector.Selector) {
					if s.Type == selector.Tag && !svcTags.Contains(s) {
						pc.Errs.Add(middleware.ErrInvalidTargetForService(svc.Name).AtGoNode(s))
					}
				})
			}
		}

		allTags.Merge(svcTags)
	}

	for _, m := range fw.GlobalMiddleware {
		m.Target.ForEach(func(s selector.Selector) {
			if s.Type == selector.Tag && !allTags.Contains(s) {
				pc.Errs.Add(middleware.ErrInvalidTargetForApp.AtGoNode(s))
			}
		})

		svc, ok := d.ServiceForPath(m.File.FSPath)
		if ok {
			pc.Errs.Add(middleware.ErrGlobalMiddlewareDefinedInService.AtGoNode(m.Decl.AST.Name, errors.AsError(fmt.Sprintf("defined in service %q", svc.Name))))
		}
	}
}
