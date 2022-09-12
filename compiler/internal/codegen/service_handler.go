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
	b.registerImports(f)

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
	b.writeValueMapper()
}

func (b *cacheKeyspaceMapperBuilder) writeKeyMapper() {
	keyType := b.schemaTypeToGoType(b.ks.KeyType)
	fn := Func().Id(b.CacheKeyspaceKeyMapperName(b.ks)).Params(
		Id("key").Add(keyType),
	).String().BlockFunc(func(g *Group) {
		_, keyIsBuiltin := b.ks.KeyType.Typ.(*schema.Type_Builtin)
		var pathLit strings.Builder
		var fmtArgs []Code

		for i, seg := range b.ks.Path.Segments {
			if i > 0 {
				pathLit.WriteString("/")
			}
			if seg.Type == paths.Literal {
				pathLit.WriteString(seg.Value)
				continue
			}

			pathLit.WriteString("%v")
			if keyIsBuiltin {
				fmtArgs = append(fmtArgs, Id("key"))
			} else {
				fmtArgs = append(fmtArgs, Id("key").Dot(seg.Value))
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

func (b *cacheKeyspaceMapperBuilder) writeValueMapper() {
	valueType := b.schemaTypeToGoType(b.ks.ValueType)
	fn := Func().Id(b.CacheKeyspaceValueMapperName(b.ks)).Params(
		Id("val").String(),
	).Params(Id("res").Add(valueType), Err().Error()).BlockFunc(func(g *Group) {
		builtin, valueIsBuiltin := b.ks.ValueType.Typ.(*schema.Type_Builtin)
		dec := b.marshaller.NewPossibleInstance("dec")
		g.Add(dec.WithFunc(func(g *Group) {
			if !valueIsBuiltin {
				panic("internal error: value mapper currently only supports builtin value types")
			}
			decodeCall, err := dec.FromStringToBuiltin(builtin.Builtin, "value", Id("val"), false)
			if err != nil {
				b.errors.Addf(b.ks.Ident().Pos(), "could not create decoder for path param, %v", err)
			}
			g.Id("res").Op("=").Add(decodeCall)
		}, func(g *Group) {
			g.Return(Id("res"), dec.LastError())
		})...)
		g.Return(Id("res"), Nil())
	})
	b.f.Add(fn)
}

func (b *Builder) CacheKeyspaceKeyMapperName(ks *est.CacheKeyspace) string {
	return fmt.Sprintf("EncoreInternal_%sKeyMapper", ks.Ident().Name)
}
func (b *Builder) CacheKeyspaceValueMapperName(ks *est.CacheKeyspace) string {
	return fmt.Sprintf("EncoreInternal_%sValueMapper", ks.Ident().Name)
}
