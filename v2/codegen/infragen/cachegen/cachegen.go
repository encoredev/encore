package cachegen

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/dave/jennifer/jen"

	"encr.dev/v2/codegen"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
)

func GenKeyspace(gen *codegen.Generator, pkg *pkginfo.Package, keyspaces []*caches.Keyspace) {
	f := gen.File(pkg, "cache")
	for idx, ks := range keyspaces {
		genKeyspaceMappers(gen, f, ks, idx)
	}
}

func genKeyspaceMappers(gen *codegen.Generator, f *codegen.File, ks *caches.Keyspace, idx int) {
	// Construct the key mapper function. We use the index since keyspaces
	// do not have resource names.
	mapper := f.FuncDecl("keyMapper", strconv.Itoa(idx))

	const input = "in"
	mapper.Params(Id(input).Add(gen.Util.Type(ks.KeyType)))
	mapper.Results(String())
	mapper.Body(Return(computePathExpression(gen.Errs, ks)))

	// Insert the label mapper configuration into the config literal.
	snippet := fmt.Sprintf("EncoreInternal_KeyMapper: %s,", mapper.Name())
	gen.Rewrite(ks.File).Insert(ks.ConfigLiteral.Lbrace+1, []byte(snippet))
}

const input = "in"

func computePathExpression(errs *perr.List, ks *caches.Keyspace) *Statement {
	structFields, isBuiltin := getStructFields(errs, ks.KeyType)
	var (
		pathLit strings.Builder
		fmtArgs []Code
	)
	for i, seg := range ks.Path.Segments {
		if i > 0 {
			pathLit.WriteString("/")
		}
		if seg.Type == caches.Literal {
			pathLit.WriteString(seg.Value)
			continue
		}

		if isBuiltin {
			verb, expr := rewriteBuiltin(structFields[builtinKey], Id(input))
			pathLit.WriteString(verb)
			fmtArgs = append(fmtArgs, expr)
		} else {
			verb, expr := rewriteBuiltin(structFields[seg.Value], Id(input).Dot(seg.Value))
			pathLit.WriteString(verb)
			fmtArgs = append(fmtArgs, expr)
		}
	}

	if len(fmtArgs) == 0 {
		// If there are no formatting arguments, return the string as a constant literal.
		return Lit(pathLit.String())
	} else {
		// Otherwise pass them to fmt.Sprintf.
		args := append([]Code{Lit(pathLit.String())}, fmtArgs...)
		return Qual("fmt", "Sprintf").Call(args...)
	}
}

// builtinKey is the key to use into the structFields map when the key is a builtin.
const builtinKey = "__builtin__"

// getStructFields resolves the struct key fields for the given keyspace.
func getStructFields(errs *perr.List, keyType schema.Type) (structFields map[string]schema.BuiltinKind, isBuiltin bool) {
	if b, ok := keyType.(schema.BuiltinType); ok {
		return map[string]schema.BuiltinKind{builtinKey: b.Kind}, true
	}

	// structFields provides a map of field names to the builtin
	// they represent. We're guaranteed these are all builtins by
	// the parser.
	structFields = make(map[string]schema.BuiltinKind)
	ref, ok := schemautil.ResolveNamedStruct(keyType, false)
	if !ok {
		errs.AddPos(keyType.ASTExpr().Pos(), "invalid cache key type: must be a named struct")
		return nil, false
	} else if ref.Pointers > 0 {
		errs.AddPos(keyType.ASTExpr().Pos(), "invalid cache key type: must not be a pointer type")
		return nil, false
	}
	st := schemautil.ConcretizeWithTypeArgs(ref.Decl.Type, ref.TypeArgs).(schema.StructType)

	for _, f := range st.Fields {
		if f.IsAnonymous() {
			errs.AddPos(f.AST.Pos(), "anonymous fields are not supported in cache keys")
			continue
		} else if f.Type.Family() != schema.Builtin {
			errs.Addf(f.AST.Pos(), "invalid cache key field %s: must be builtin",
				f.Name.MustGet())
			continue
		}

		structFields[f.Name.MustGet()] = f.Type.(schema.BuiltinType).Kind
	}
	return structFields, false
}

// rewriteBuiltin returns the code to rewrite a builtin type for use as a cache key.
func rewriteBuiltin(kind schema.BuiltinKind, expr Code) (verb string, rewritten Code) {
	switch kind {
	case schema.String:
		return "%s", Qual("strings", "ReplaceAll").Call(expr, Lit("/"), Lit(`\/`))
	case schema.Bytes:
		return "%s", Qual("bytes", "ReplaceAll").Call(
			expr,
			Index().Byte().Parens(Lit("/")),
			Index().Byte().Parens(Lit(`\/`)),
		)
	default:
		return "%v", expr
	}
}
