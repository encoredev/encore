package pubsubgen

import (
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/resource"
)

func Gen(gen *codegen.Generator, pkg *pkginfo.Package, appDesc *app.Desc, subs []*pubsub.Subscription) {
	var file *codegen.File // created on first use
	for _, sub := range subs {
		if method, ok := sub.MethodHandler.Get(); ok {
			if file == nil {
				file = gen.File(pkg, "pubsub")
			}
			rewriteMethodHandler(gen, file, appDesc, sub, method)
		}
	}
}

func rewriteMethodHandler(gen *codegen.Generator, f *codegen.File, appDesc *app.Desc, sub *pubsub.Subscription, method pubsub.MethodHandler) {
	res, ok := appDesc.Parse.ResourceForQN(sub.Topic).Get()
	if !ok || res.Kind() != resource.PubSubTopic {
		gen.Errs.Add(
			pubsub.ErrSubscriptionTopicNotResource.AtGoNode(sub.AST.Args[0]),
		)
		return
	}
	topic := res.(*pubsub.Topic)

	svc, ok := appDesc.ServiceForPath(sub.File.FSPath)
	if !ok {
		gen.Errs.Add(
			pubsub.ErrInvalidMethodHandler.AtGoNode(sub.Handler),
		)
		return
	}

	handler := f.FuncDecl("handler", strings.ReplaceAll(sub.Name, "-", "_"))

	handler.Params(Id("ctx").Qual("context", "Context"), Id("msg").Add(gen.Util.Type(topic.MessageType.ToType())))
	handler.Results(Error())

	handler.BodyFunc(func(g *Group) {
		// svc, err := service.Get[*SvcStruct]()
		g.List(Id("svc"), Err()).Op(":=").Qual("encore.dev/appruntime/apisdk/service", "Get").Types(
			Op("*").Id(method.Decl.Name),
		).Call(Lit(svc.Name))
		// if err != nil { return err }
		g.If(Err().Op("!=").Nil()).Block(
			Return(Err()),
		)
		// return svc.Method(ctx, msg)
		g.Return(Id("svc").Dot(method.Method).Call(Id("ctx"), Id("msg")))
	})

	gen.Rewrite(sub.File).Replace(sub.Handler.Pos(), sub.Handler.End(), []byte(handler.Name()))
}
