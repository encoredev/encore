package app

import (
	"go/ast"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/apis/api"
	"encr.dev/v2/parser/infra/pubsub"
)

func (d *Desc) validatePubSub(pc *parsectx.Context, result *parser.Result) {
	type topic struct {
		resource *pubsub.Topic
		subs     map[string]*pubsub.Subscription
	}
	topics := make(map[string]topic)
	topicsByBinding := make(map[pkginfo.QualifiedName]string)

	var subs []*pubsub.Subscription

	for _, res := range d.Parse.Resources() {
		switch res := res.(type) {
		case *pubsub.Topic:
			if existing, ok := topics[res.Name]; ok {
				pc.Errs.Add(pubsub.ErrTopicNameNotUnique.
					AtGoNode(existing.resource.AST.Args[0], errors.AsHelp("originally defined here")).
					AtGoNode(res.AST.Args[0], errors.AsError("duplicated here")),
				)
			} else {
				topics[res.Name] = topic{
					resource: res,
					subs:     make(map[string]*pubsub.Subscription),
				}

				for _, bind := range d.Parse.PkgDeclBinds(res) {
					topicsByBinding[bind.QualifiedName()] = res.Name
				}
			}
		case *pubsub.Subscription:
			subs = append(subs, res)
		}
	}

	for _, sub := range subs {
		topicName := topicsByBinding[sub.Topic]
		topic, ok := topics[topicName]
		if !ok {
			pc.Errs.Add(pubsub.ErrSubscriptionTopicNotResource.AtGoNode(sub.AST.Args[0]))
			continue
		}

		if existing, ok := topic.subs[sub.Name]; ok {
			pc.Errs.Add(pubsub.ErrSubscriptionNameNotUnique.
				AtGoNode(existing.AST.Args[1], errors.AsHelp("originally defined here")).
				AtGoNode(sub.AST.Args[1], errors.AsError("duplicated here")),
			)
		} else {
			topic.subs[sub.Name] = sub
		}

		subService, ok := d.ServiceForPath(sub.File.FSPath)
		if !ok {
			pc.Errs.Add(pubsub.ErrUnableToIdentifyServicesInvolved.AtGoNode(sub, errors.AsError("unable to identify service for subscription")))
		}

		// Verify the handler is ok
		handlerIsAPIEndpoint := false
		if handlerUsage, ok := result.UsageFromNode(sub.Handler).Get(); ok {
			if endpointUsage, ok := handlerUsage.(*api.ReferenceUsage); ok {
				ep := endpointUsage.Endpoint

				endpointService, ok := d.ServiceForPath(ep.File.FSPath)
				if !ok {
					pc.Errs.Add(pubsub.ErrUnableToIdentifyServicesInvolved.AtGoNode(ep, errors.AsError("unable to identify service for endpoint")))
				}

				if endpointService != subService {
					pc.Errs.Add(
						pubsub.ErrSubscriptionHandlerNotDefinedInSameService.
							AtGoNode(sub.Handler, errors.AsError("handler specified here")).
							AtGoNode(ep.Decl.AST.Name, errors.AsHelp("endpoint defined here")),
					)
				}

				handlerIsAPIEndpoint = true
			}
		}

		if !handlerIsAPIEndpoint {
			qn, ok := sub.File.Names().ResolvePkgLevelRef(sub.Handler)
			if ok {
				if pkg, ok := result.PackageAt(qn.PkgPath).Get(); ok {
					funcName := option.Map(pkg.Names().FuncDecl(qn.Name), func(f *ast.FuncDecl) *ast.Ident {
						return f.Name
					}).GetOrElse(nil)

					if svc, ok := d.ServiceForPath(pkg.FSPath); ok {
						if svc != subService {
							pc.Errs.Add(
								pubsub.ErrSubscriptionHandlerNotDefinedInSameService.
									AtGoNode(sub.Handler, errors.AsError("handler specified here")).
									AtGoNode(funcName, errors.AsHelp("handler function defined here")),
							)
						}
					} else {
						pc.Errs.Add(
							pubsub.ErrSubscriptionHandlerNotDefinedInSameService.
								AtGoNode(sub.Handler, errors.AsError("handler specified here")).
								AtGoNode(funcName, errors.AsHelp("handler function defined here")),
						)
					}
				} else {
					pc.Errs.Add(
						pubsub.ErrUnableToIdentifyServicesInvolved.
							AtGoNode(sub.Handler, errors.AsError("unable to identify package for this reference")),
					)
				}
			} else {
				// this is ok, because it means we're not dealing with an ident or selector - thus
				// it can't be a valid funciton reference and the normal Go compiler will pick this up
			}

		}
	}
}
