package app

import (
	"fmt"

	"encr.dev/pkg/errors"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/api/apipaths"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/servicestruct"
)

func (d *Desc) validateAPIs(pc *parsectx.Context, fw *apiframework.AppDesc) {

	apiPaths := apipaths.NewSet()

	for _, svc := range d.Services {
		fwSvc, ok := svc.Framework.Get()
		if !ok {
			continue
		}

		svcStruct, hasSvcStruct := fwSvc.ServiceStruct.Get()

		for _, ep := range fwSvc.Endpoints {
			// Check if an auth handler is defined for an endpoint that requires auth.
			if ep.Access == api.Auth && fw.AuthHandler.Empty() {
				pc.Errs.Add(
					errors.AtOptionalNode(authhandler.ErrNoAuthHandlerDefined, ep.AccessField),
				)
			}

			// Check for duplicate paths by adding them to the set
			// Note, errors will be reported automatically to pc.Errs
			for _, method := range ep.HTTPMethods {
				apiPaths.Add(pc.Errs, method, ep.Path)
			}

			if receiver, ok := ep.Recv.Get(); ok {
				if !hasSvcStruct {
					pc.Errs.Add(
						servicestruct.ErrReceiverNotAServiceStruct.
							AtGoNode(receiver.AST, errors.AsError("there are no service structs defined in this service")),
					)
				} else if receiver.Decl != svcStruct.Decl {
					pc.Errs.Add(
						servicestruct.ErrReceiverNotAServiceStruct.
							AtGoNode(receiver.AST, errors.AsError(
								fmt.Sprintf("try changing this to `*%s`", svcStruct.Decl.Name),
							)).
							AtGoNode(svcStruct.Decl.AST.Name, errors.AsHelp(
								"this is the service struct for this service",
							)),
					)
				}
			}
		}
	}
}
