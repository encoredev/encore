package codegen

import (
	"fmt"
	"sort"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
	"encr.dev/parser/paths"
	"encr.dev/pkg/idents"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func (b *Builder) Infra(pkg *est.Package) (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	f = NewFilePathName(pkg.ImportPath, pkg.Name)
	b.registerImports(f, pkg.ImportPath)

	// Import the runtime package with '_' as its name to start with to ensure it's imported.
	// If other code uses it will be imported under its proper name.
	f.Anon("encore.dev/appruntime/app/appinit")

	for _, res := range pkg.Resources {
		switch res.Type() {
		case est.CacheKeyspaceResource:
			f.Line()
			ks := res.(*est.CacheKeyspace)
			b.buildCacheKeyspaceMappers(f, ks)
		case est.MetricResource:
			if m := res.(*est.Metric); m.LabelsType != nil {
				f.Line()
				b.buildMetricLabelMappers(f, m)
			}
		}
	}

	return f, b.errors.Err()
}

func (b *Builder) buildCacheKeyspaceMappers(f *File, ks *est.CacheKeyspace) {
	bb := &cacheKeyspaceMapperBuilder{
		Builder: b,
		f:       f,
		ks:      ks,
	}
	bb.Write()
}

type cacheKeyspaceMapperBuilder struct {
	*Builder
	f  *File
	ks *est.CacheKeyspace
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

func (b *Builder) buildMetricLabelMappers(f *File, m *est.Metric) {
	bb := &metricLabelMapperBuilder{
		Builder: b,
		f:       f,
		m:       m,
	}
	bb.Write()
}

type metricLabelMapperBuilder struct {
	*Builder
	f *File
	m *est.Metric
}

func (b *metricLabelMapperBuilder) Write() {
	b.writeLabelMapper()
}

func (b *metricLabelMapperBuilder) writeLabelMapper() {
	labelsType := b.schemaTypeToGoType(b.m.LabelsType)
	fn := Func().Id(b.MetricLabelMapperName(b.m)).Params(
		Id("key").Add(labelsType),
	).Index().Qual("encore.dev/metrics", "KeyValue").Block(
		Return(Index().Qual("encore.dev/metrics", "KeyValue").ValuesFunc(func(g *Group) {
			formatFieldValue := func(f *schema.Field) Code {
				val := Id("key").Dot(f.Name)
				switch builtin := f.Typ.GetBuiltin(); builtin {
				case schema.Builtin_STRING:
					return val
				case schema.Builtin_BOOL:
					return Qual("strconv", "FormatBool").Call(val)
				case schema.Builtin_INT, schema.Builtin_INT64, schema.Builtin_INT32, schema.Builtin_INT16, schema.Builtin_INT8:
					cast := builtin != schema.Builtin_INT64
					if cast {
						val = Int64().Call(val)
					}
					return Qual("strconv", "FormatInt").Call(val, Lit(10))
				case schema.Builtin_UINT, schema.Builtin_UINT64, schema.Builtin_UINT32, schema.Builtin_UINT16, schema.Builtin_UINT8:
					cast := builtin != schema.Builtin_UINT64
					if cast {
						val = Uint64().Call(val)
					}
					return Qual("strconv", "FormatUint").Call(val, Lit(10))
				default:
					b.errorf("invalid metric label field %s: unsupported builtin type %v", f.Name, builtin)
					return Lit("")
				}
			}

			decl := b.m.LabelsType.GetNamed()
			st := b.res.App.Decls[decl.Id].Type.GetStruct()

			// Sort the fields by name so the output doesn't depend on
			// field order.
			fields := make([]*schema.Field, len(st.Fields))
			copy(fields, st.Fields)
			sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })

			for _, f := range fields {
				key := idents.Convert(f.Name, idents.SnakeCase)
				g.Add(Values(Dict{
					Id("Key"):   Lit(key),
					Id("Value"): formatFieldValue(f),
				}))
			}
		})),
	)
	b.f.Add(fn)
}

func (b *Builder) MetricLabelMapperName(m *est.Metric) string {
	return fmt.Sprintf("EncoreInternal_%sLabelMapper", m.Ident().Name)
}
