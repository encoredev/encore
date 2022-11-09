package codegen

import (
	"fmt"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func (b *Builder) ServiceHandlers(svc *est.Service) (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	f = NewFilePathName(svc.Root.ImportPath, svc.Name)
	b.registerImports(f, svc.Root.ImportPath)

	// Import the runtime package with '_' as its name to start with to ensure it's imported.
	// If other code uses it will be imported under its proper name.
	f.Anon("encore.dev/appruntime/app/appinit")

	if svc.Struct != nil {
		f.Line()
		b.buildServiceStructHandler(f, svc.Struct)
	}

	for _, rpc := range svc.RPCs {
		f.Line()
		b.buildRPC(f, svc, rpc)
	}

	for _, pkg := range svc.Pkgs {
		for _, res := range pkg.Resources {
			if res.Type() == est.CacheKeyspaceResource {
				f.Line()
				ks := res.(*est.CacheKeyspace)
				b.buildCacheKeyspaceMappers(f, svc, ks)
			}
		}
	}

	if ah := b.res.App.AuthHandler; ah != nil && ah.Svc == svc {
		f.Line()
		b.buildAuthHandler(f, ah)
	}

	return f, b.errors.Err()
}

func (b *Builder) buildServiceStructHandler(f *File, ss *est.ServiceStruct) {
	bb := &serviceStructHandlerBuilder{
		Builder: b,
		f:       f,
		svc:     ss.Svc,
		ss:      ss,
	}
	bb.Write()
}

type serviceStructHandlerBuilder struct {
	*Builder
	f   *File
	svc *est.Service
	ss  *est.ServiceStruct
}

func (b *serviceStructHandlerBuilder) Write() {
	initFuncName := Nil()
	if b.ss.Init != nil {
		initFuncName = Id(b.ss.Init.Name.Name)
	}

	setupDefLoc := 0
	if b.ss.Init != nil {
		setupDefLoc = int(b.res.Nodes[b.svc.Root][b.ss.Init].Id)
	}

	handler := Var().Id(b.serviceStructName(b.ss)).Op("=").Op("&").Qual("encore.dev/appruntime/service", "Decl").Types(
		Id(b.ss.Name),
	).Custom(Options{
		Open:      "{",
		Close:     "}",
		Separator: ",",
		Multi:     true,
	},
		Id("Service").Op(":").Lit(b.svc.Name),
		Id("Name").Op(":").Lit(b.ss.Name),
		Id("Setup").Op(":").Add(initFuncName),
		Id("SetupDefLoc").Op(":").Lit(setupDefLoc),
	)
	b.f.Add(handler)
}

func (b *Builder) serviceStructName(ss *est.ServiceStruct) string {
	return fmt.Sprintf("EncoreInternal_%sService", ss.Name)
}

func (b *Builder) buildCacheKeyspaceMappers(f *File, svc *est.Service, ks *est.CacheKeyspace) {
	bb := &cacheKeyspaceMapperBuilder{
		Builder: b,
		f:       f,
		svc:     svc,
		ks:      ks,
	}
	bb.Write()
}

type cacheKeyspaceMapperBuilder struct {
	*Builder
	f   *File
	svc *est.Service
	ks  *est.CacheKeyspace
}

func (b *cacheKeyspaceMapperBuilder) Write() {
	b.writeKeyMapper()
}

func (b *cacheKeyspaceMapperBuilder) writeKeyMapper() {
	keyType := b.schemaTypeToGoType(b.ks.KeyType)
	fn := Func().Id(b.CacheKeyspaceKeyMapperName(b.ks)).Params(
		Id("key").Add(keyType),
	).String().BlockFunc(func(g *Group) {
		keyType, keyIsBuiltin := b.ks.KeyType.Typ.(*schema.Type_Builtin)
		var pathLit strings.Builder
		var fmtArgs []Code

		rewriteBuiltin := func(builtin schema.Builtin, expr Code) (verb string, rewritten Code) {
			switch builtin {
			case schema.Builtin_STRING:
				return "%s", Qual("strings", "ReplaceAll").Call(expr, Lit("/"), Lit(`\/`))
			case schema.Builtin_BYTES:
				return "%s", Qual("bytes", "ReplaceAll").Call(
					expr,
					Index().Byte().Parens(Lit("/")),
					Index().Byte().Parens(Lit(`\/`)),
				)
			default:
				return "%v", expr
			}
		}

		// structFields provides a map of field names to the builtin
		// they represent. We're guaranteed these are all builtins by
		// the parser.
		structFields := make(map[string]schema.Builtin)
		if !keyIsBuiltin {
			decl := b.ks.KeyType.GetNamed()
			st := b.res.App.Decls[decl.Id].Type.GetStruct()
			for _, f := range st.Fields {
				structFields[f.Name] = f.Typ.GetBuiltin()
			}
		}

		for i, seg := range b.ks.Path.Segments {
			if i > 0 {
				pathLit.WriteString("/")
			}
			if seg.Type == paths.Literal {
				pathLit.WriteString(seg.Value)
				continue
			}

			if keyIsBuiltin {
				verb, expr := rewriteBuiltin(keyType.Builtin, Id("key"))
				pathLit.WriteString(verb)
				fmtArgs = append(fmtArgs, expr)
			} else {
				verb, expr := rewriteBuiltin(structFields[seg.Value], Id("key").Dot(seg.Value))
				pathLit.WriteString(verb)
				fmtArgs = append(fmtArgs, expr)
			}
		}

		if len(fmtArgs) == 0 {
			g.Return(Lit(pathLit.String()))
		} else {
			args := append([]Code{Lit(pathLit.String())}, fmtArgs...)
			g.Return(Qual("fmt", "Sprintf").Call(args...))
		}
	})
	b.f.Add(fn)
}

func (b *Builder) CacheKeyspaceKeyMapperName(ks *est.CacheKeyspace) string {
	return fmt.Sprintf("EncoreInternal_%sKeyMapper", ks.Ident().Name)
}
