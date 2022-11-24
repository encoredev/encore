package codegen

import (
	"fmt"
	"net/http"
	"path"
	"sort"

	. "github.com/dave/jennifer/jen"

	"encr.dev/internal/gocodegen"
	"encr.dev/parser"
	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
	"encr.dev/pkg/errlist"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

const JsonPkg = "github.com/json-iterator/go"

type Builder struct {
	res *parser.Result

	marshaller *gocodegen.MarshallingCodeGenerator
	errors     *errlist.List
}

func NewBuilder(res *parser.Result) *Builder {
	marshallerPkgPath := path.Join(res.Meta.ModulePath, "__encore", "etype")
	marshaller := gocodegen.NewMarshallingCodeGenerator(marshallerPkgPath, "Marshaller", false)

	return &Builder{
		res:        res,
		errors:     errlist.New(res.FileSet),
		marshaller: marshaller,
	}
}

func (b *Builder) Main(compilerVersion string, mainPkgPath, mainFuncName string) (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	if mainPkgPath != "" {
		f = NewFilePathName(mainPkgPath, "main")
	} else {
		f = NewFile("main")
	}

	b.registerImports(f, mainPkgPath)
	b.importServices(f, mainPkgPath)

	mwNames, mwCode := b.RenderMiddlewares(mainPkgPath)

	corsHeaders, err := b.computeCORSHeaders()
	if err != nil {
		b.error(eerror.Wrap(err, "codegen", "failed to compute CORS headers", nil))
	}

	f.Anon("unsafe") // for go:linkname
	f.Comment("loadApp loads the Encore app runtime.")
	f.Comment("//go:linkname loadApp encore.dev/appruntime/app/appinit.load")
	f.Func().Id("loadApp").Params().Op("*").Qual("encore.dev/appruntime/app/appinit", "LoadData").BlockFunc(func(g *Group) {
		g.Id("static").Op(":=").Op("&").Qual("encore.dev/appruntime/config", "Static").Values(Dict{
			Id("AuthData"):       b.authDataType(),
			Id("EncoreCompiler"): Lit(compilerVersion),
			Id("AppCommit"): Qual("encore.dev/appruntime/config", "CommitInfo").Values(Dict{
				Id("Revision"):    Lit(b.res.Meta.AppRevision),
				Id("Uncommitted"): Lit(b.res.Meta.UncommittedChanges),
			}),
			Id("CORSHeaders"):  corsHeaders,
			Id("PubsubTopics"): b.computeStaticPubsubConfig(),
			Id("Testing"):      False(),
			Id("TestService"):  Lit(""),
		})
		g.Id("handlers").Op(":=").Add(b.computeHandlerRegistrationConfig(mwNames))

		authHandlerExpr := Nil()
		if ah := b.res.App.AuthHandler; ah != nil {
			authHandlerExpr = Qual(ah.Svc.Root.ImportPath, b.authHandlerName(ah))
		}

		g.Return(Op("&").Qual("encore.dev/appruntime/app/appinit", "LoadData").Values(Dict{
			Id("StaticCfg"):   Id("static"),
			Id("APIHandlers"): Id("handlers"),
			Id("AuthHandler"): authHandlerExpr,
		}))
	})
	f.Line()

	if mainFuncName == "" {
		mainFuncName = "main"
	}

	f.Func().Id(mainFuncName).Params().Block(
		Qual("encore.dev/appruntime/app/appinit", "AppMain").Call(),
	)

	for _, c := range mwCode {
		f.Line()
		f.Add(c)
	}

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

func (b *Builder) computeCORSHeaders() (Code, error) {
	// Find all used headers
	usedHeaderSet := map[string]struct{}{}
	usedHeaders := []string{}

	// Walk all the structures looking for headers
	decls := b.res.Meta.Decls
	for _, param := range b.res.App.ParamTypes() {
		err := schema.Walk(decls, param, func(node any) error {
			switch n := node.(type) {
			case *schema.Struct:
				for _, field := range n.Fields {
					for _, tag := range field.Tags {
						if tag.Key == "header" {
							name := http.CanonicalHeaderKey(tag.Name)
							if _, found := usedHeaderSet[name]; !found {
								usedHeaderSet[name] = struct{}{}
								usedHeaders = append(usedHeaders, name)
							}
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if len(usedHeaders) == 0 {
		return Nil(), nil
	}

	// Generate the code list of used static headers
	sort.Strings(usedHeaders)
	usedHeadersCode := make([]Code, len(usedHeaders))
	for _, header := range usedHeaders {
		usedHeadersCode = append(usedHeadersCode, Lit(header))
	}
	return Index().String().Values(usedHeadersCode...), nil
}

func (b *Builder) computeHandlerRegistrationConfig(mwNames map[*est.Middleware]*Statement) *Statement {
	return Index().Qual("encore.dev/appruntime/api", "HandlerRegistration").CustomFunc(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	}, func(g *Group) {
		var globalMW []*est.Middleware
		for _, mw := range b.res.App.Middleware {
			if mw.Global {
				globalMW = append(globalMW, mw)
			}
		}

		for _, svc := range b.res.App.Services {
			for _, rpc := range svc.RPCs {
				// Compute middleware for this service.
				rpcMW := b.res.App.MatchingMiddleware(rpc)
				mwExpr := Nil()
				if len(rpcMW) > 0 {
					mwExpr = Index().Op("*").Qual("encore.dev/appruntime/api", "Middleware").ValuesFunc(func(g *Group) {
						for _, mw := range rpcMW {
							g.Add(mwNames[mw])
						}
					})
				}

				g.Values(Dict{
					Id("Handler"):    Qual(svc.Root.ImportPath, b.rpcHandlerName(rpc)),
					Id("Middleware"): mwExpr,
				})
			}
		}
	})
}

func (b *Builder) importServices(f *File, mainPkgPath string) {
	// All services should be imported by the main package so they get initialized on system startup
	// Services may not have API handlers as they could be purely operating on PubSub subscriptions
	// so without this anonymous package import, that service might not be initialised.
	for _, svc := range b.res.App.Services {
		if svc.Root.ImportPath != mainPkgPath {
			f.Anon(svc.Root.ImportPath)
		}
	}
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
		typName := b.schemaTypeToGoType(derefPointer(ah.AuthData.Type))
		if ah.AuthData.IsPointer() {
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

func (b bailout) String() string {
	return fmt.Sprintf("bailout(%s)", b.err)
}
