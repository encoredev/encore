package metricsgen

import (
	"fmt"

	. "github.com/dave/jennifer/jen"

	"encr.dev/pkg/idents"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/metrics"
)

func Gen(gen *codegen.Generator, pkg *pkginfo.Package, metrics []*metrics.Metric) {
	f := gen.File(pkg, "metrics")
	for _, m := range metrics {
		genLabelMapper(gen, f, m)
	}
}

func genLabelMapper(gen *codegen.Generator, f *codegen.File, m *metrics.Metric) {
	// If there is no label type there's nothing to do.
	if m.LabelType.Empty() {
		return
	}
	labelType := m.LabelType.MustGet()

	declRef, ok := schemautil.ResolveNamedStruct(labelType, false)
	if !ok {
		gen.Errs.AddPos(labelType.ASTExpr().Pos(), "invalid metric label type: must be a named struct")
		return
	} else if declRef.Pointers > 0 {
		gen.Errs.AddPos(labelType.ASTExpr().Pos(), "invalid metric label type: must not be a pointer type")
		return
	}
	concrete := schemautil.ConcretizeWithTypeArgs(declRef.Decl.Type, declRef.TypeArgs).(schema.StructType)

	mapper := f.FuncDecl("labelMapper", m.Name)

	const input = "in"
	mapper.Params(Id(input).Add(gen.Util.Type(labelType)))
	mapper.Results(Index().Qual("encore.dev/metrics", "KeyValue"))

	mapper.Body(
		Return(Index().Qual("encore.dev/metrics", "KeyValue").ValuesFunc(func(g *Group) {
			for _, f := range concrete.Fields {
				if f.IsAnonymous() {
					gen.Errs.AddPos(f.AST.Pos(), "anonymous fields are not supported in metric labels")
					continue
				}

				key := idents.Convert(f.Name.MustGet(), idents.SnakeCase)
				g.Add(Values(Dict{
					Id("Key"):   Lit(key),
					Id("Value"): fieldToString(gen.Errs, f, Id(input).Dot(f.Name.MustGet())),
				}))
			}
		})),
	)

	// Insert the label mapper configuration into the metrics config literal.
	snippet := fmt.Sprintf("EncoreInternal_LabelMapper: %s,", mapper.Name())
	gen.Rewrite(m.File).Insert(m.ConfigLiteral.Lbrace+1, []byte(snippet))
}

// fieldToString returns the code to convert the given value of the given builtin type to a string.
// If the field is not a valid builtin or is anonymous, it reports an error.
func fieldToString(errs *perr.List, field schema.StructField, val Code) Code {
	fieldName := field.Name.MustGet()

	typ, ok := field.Type.(schema.BuiltinType)
	if !ok {
		errs.Addf(field.AST.Pos(), "invalid metric label field %s: must be string, bool, or integer type",
			fieldName)
		return Null()
	}

	kind := typ.Kind
	switch true {
	case kind == schema.String:
		return val
	case kind == schema.Bool:
		return Qual("strconv", "FormatBool").Call(val)

	case schemautil.IsBuiltinKind(typ, schemautil.Signed...):
		cast := kind != schema.Int64
		if cast {
			val = Int64().Call(val)
		}
		return Qual("strconv", "FormatInt").Call(val, Lit(10))

	case schemautil.IsBuiltinKind(typ, schemautil.Unsigned...):
		cast := kind != schema.Uint64
		if cast {
			val = Uint64().Call(val)
		}
		return Qual("strconv", "FormatUint").Call(val, Lit(10))

	default:
		errs.Addf(field.AST.Pos(), "invalid metric label field %s: must be string, bool, or integer type",
			fieldName)
		return Null()
	}
}
