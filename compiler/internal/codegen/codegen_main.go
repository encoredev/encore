package codegen

import (
	"fmt"
	"path"

	. "github.com/dave/jennifer/jen"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser"
	"encr.dev/parser/est"
	"encr.dev/pkg/errlist"
)

const JsonPkg = "github.com/json-iterator/go"

type Builder struct {
	res             *parser.Result
	compilerVersion string

	marshaller *gocodegen.MarshallingCodeGenerator
	errors     *errlist.List
}

func NewBuilder(res *parser.Result, compilerVersion string) *Builder {
	marshallerPkgPath := path.Join(res.Meta.ModulePath, "__encore", "etype")
	marshaller := gocodegen.NewMarshallingCodeGenerator(marshallerPkgPath, "Marshaller", false)

	return &Builder{
		res:             res,
		compilerVersion: compilerVersion,
		errors:          errlist.New(res.FileSet),
		marshaller:      marshaller,
	}
}

func (b *Builder) Main() (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	f = NewFile("main")
	b.registerImports(f)

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadApp loads the Encore app runtime.")
	f.Comment("//go:linkname loadApp encore.dev/appruntime/app.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app", "LoadData").BlockFunc(func(g *Group) {
		g.Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):       b.authDataType(),
			Id("EncoreCompiler"): Lit(b.compilerVersion),
			Id("AppCommit"): Qual("encore.dev/appruntime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(b.res.Meta.AppRevision),
				Id("Uncommitted"): Lit(b.res.Meta.UncommittedChanges),
			}),
			Id("PubsubTopics"): b.computeStaticPubsubConfig(),
			Id("Testing"):      False(),
			Id("TestService"):  Lit(""),
		})
		g.Id("handlers").Op(":=").Index().Qual("encore.dev/appruntime/api", "Handler").CustomFunc(Options{
			Open:      "{",
			Close:     "}",
			Separator: ",",
			Multi:     true,
		}, func(g *Group) {
			for _, svc := range b.res.App.Services {
				for _, rpc := range svc.RPCs {
					g.Add(Qual(svc.Root.ImportPath, b.rpcHandlerName(rpc)))
				}
			}
		})

		authHandlerExpr := Nil()
		if ah := b.res.App.AuthHandler; ah != nil {
			authHandlerExpr = Qual(ah.Svc.Root.ImportPath, b.authHandlerName(ah))
		}

		g.Return(Op("&").Qual("encore.dev/appruntime/app", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Id("handlers"),
			Id("AuthHandler"): authHandlerExpr,
		}))
	})
	f.Line()

	f.Func().Id("main").Params().Block(
		Qual("encore.dev/appruntime/app", "Main").Call(),
	)

	return f, b.errors.Err()
}

func (b *Builder) computeStaticPubsubConfig() Code {
	pubsubTopicDict := Dict{}
	for _, topic := range b.res.App.PubSubTopics {
		subscriptions := Dict{}

		for _, sub := range topic.Subscribers {
			traceID := int(b.res.Nodes[sub.DeclFile.Pkg][sub.IdentAST].Id)

			subscriptions[Lit(sub.Name)] = Values(Dict{
				Id("Service"):  Lit(sub.DeclFile.Pkg.Service.Name),
				Id("TraceIdx"): Lit(traceID),
			})
		}

		pubsubTopicDict[Lit(topic.Name)] = Values(Dict{
			Id("Subscriptions"): Map(String()).Op("*").Qual("encore.dev/appruntime/config", "StaticPubsubSubscription").Values(subscriptions),
		})
	}
	return Map(String()).Op("*").Qual("encore.dev/appruntime/config", "StaticPubsubTopic").Values(pubsubTopicDict)
}

func (b *Builder) typeName(param *est.Param, skipPtr bool) *Statement {
	typName := b.schemaTypeToGoType(param.Type)

	if param.IsPtr && !skipPtr {
		typName = Op("*").Add(typName)
	}

	return typName
}

func (b *Builder) authDataType() Code {
	if ah := b.res.App.AuthHandler; ah != nil && ah.AuthData != nil {
		typName := b.schemaTypeToGoType(ah.AuthData.Type)
		if ah.AuthData.IsPtr {
			// reflect.TypeOf((*T)(nil))
			return Qual("reflect", "TypeOf").Call(Parens(Op("*").Add(typName)).Call(Nil()))
		} else {
			// reflect.TypeOf(T{})
			return Qual("reflect", "TypeOf").Call(typName.Values())
		}
	}
	return Nil()
}

func (b *Builder) error(err error) {
	panic(bailout{err})
}

func (b *Builder) errorf(format string, args ...interface{}) {
	panic(bailout{fmt.Errorf(format, args...)})
}

type bailout struct {
	err error
}
