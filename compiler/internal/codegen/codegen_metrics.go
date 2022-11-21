package codegen

import (
	"fmt"
	"sort"

	. "github.com/dave/jennifer/jen"

	"encr.dev/parser/est"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// PackageResources generates code for packages with resources.
func (b *Builder) PackageResources(pkg *est.Package) (f *File, err error) {
	defer b.errors.HandleBailout(&err)

	f = NewFilePathName(pkg.ImportPath, pkg.Name)
	b.registerImports(f)

	// Import the runtime package to force this package to have a dependency
	// on the runtime, to ensure proper initialization order.
	f.Anon("encore.dev/appruntime/app/appinit")

	for _, res := range pkg.Resources {
		if res.Type() == est.MetricResource {
			if m := res.(*est.Metric); m.LabelsType != nil {
				f.Line()
				b.buildMetricLabelMappers(f, m)
			}
		}
	}

	return f, b.errors.Err()
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
	).String().Block(
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
				g.Add(Values(Dict{
					Id("Key"):   Lit(f.Name),
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
