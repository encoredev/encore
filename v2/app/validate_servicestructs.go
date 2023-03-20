package app

import (
	"fmt"

	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/servicestruct"
)

func (d *Desc) validateServiceStructs(pc *parsectx.Context, result *parser.Result) {
	for _, svc := range d.Services {
		if fwSvc, ok := svc.Framework.Get(); ok {
			if ss, ok := fwSvc.ServiceStruct.Get(); ok {
				for _, use := range result.Usages(ss) {
					refText := "referenced here"
					useSvc, ok := d.ServiceForPath(use.DeclaredIn().FSPath)
					if ok {
						refText = fmt.Sprintf("referenced in service %q", useSvc.Name)
					}

					if !use.DeclaredIn().FSPath.HasPrefix(svc.FSRoot) {
						pc.Errs.Add(
							servicestruct.ErrServiceStructReferencedInAnotherService.
								AtGoNode(use, errors.AsError(refText)).
								AtGoNode(ss.Decl.AST.Name, errors.AsHelp(fmt.Sprintf("defined in service %q", svc.Name))),
						)
					}
				}
			}
		}
	}
}
