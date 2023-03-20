package app

import (
	"fmt"

	"encr.dev/pkg/errors"
	"encr.dev/v2/app/apiframework"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/resourcepaths"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/apis/authhandler"
	"encr.dev/v2/parser/apis/servicestruct"
	"encr.dev/v2/parser/infra/crons"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/resource"
)

func (d *Desc) validateAPIs(pc *parsectx.Context, fw *apiframework.AppDesc, result *parser.Result) {

	apiPaths := resourcepaths.NewSet()

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

			if ep.Raw {
				for _, rawUsage := range result.Usages(ep) {
					pc.Errs.Add(
						api.ErrRawEndpointsCannotBeCalled.
							AtGoNode(rawUsage, errors.AsError("used here")).
							AtGoNode(ep.Decl.AST.Name, errors.AsHelp("defined here")),
					)
				}
			}

			// Check for usages outside of services
			for _, invalidUsage := range d.ResourceUsageOutsideServices[ep] {
				pc.Errs.Add(
					api.ErrAPICalledOutsideService.
						AtGoNode(invalidUsage, errors.AsError("called here")).
						AtGoNode(ep.Decl.AST.Name, errors.AsHelp("defined here")),
				)
			}

			// Check for invalid references
			for _, usage := range result.Usages(ep) {
				switch usage := usage.(type) {
				case *api.ReferenceUsage:
					// API's can only be referenced
					isValid := result.ResourceConstructorContaining(usage).Contains(func(res resource.Resource) bool {
						switch res := res.(type) {
						case *pubsub.Subscription:
							return res.Handler == usage.Ref
						case *crons.Job:
							return res.EndpointAST == usage.Ref
						default:
							return false
						}
					})

					if !isValid {
						pc.Errs.Add(
							api.ErrInvalidEndpointUsage.
								AtGoNode(usage, errors.AsError("used here")).
								AtGoNode(ep.Decl.AST.Name, errors.AsHelp("defined here")),
						)
					}
				}
			}
		}
	}
}
