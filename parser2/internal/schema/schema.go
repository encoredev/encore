// Package schema implements parsing of Go types into Encore's schema format.
package schema

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"sync"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/paths"
	"encr.dev/parser2/internal/perr"
	"encr.dev/parser2/internal/pkginfo"
)

// NewParser constructs a new schema parser.
func NewParser(c *parsectx.Context, l *pkginfo.Loader) *Parser {
	return &Parser{
		c:     c,
		l:     l,
		decls: make(map[declKey]*Decl),
	}
}

// Parser parses Go types into Encore's schema format.
type Parser struct {
	c *parsectx.Context
	l *pkginfo.Loader

	declsMu sync.Mutex
	decls   map[declKey]*Decl // pkg/path.Name -> decl
}

// ParseType parses the schema from a type expression.
func (p *Parser) ParseType(file *pkginfo.File, expr ast.Expr) Type {
	r := p.newTypeResolver(nil, nil)
	return r.parseType(file, expr)
}

// newTypeResolver is a helper function to create a new typeResolver.
func (p *Parser) newTypeResolver(decl *Decl, typeParamsInScope map[string]int) *typeResolver {
	return &typeResolver{
		p:                 p,
		errs:              p.c.Errs,
		decl:              decl,
		typeParamsInScope: typeParamsInScope,
	}
}

// typeResolver resolves types from AST expressions.
type typeResolver struct {
	// p is the parser that created this type resolver.
	p    *Parser
	errs *perr.List

	// decl is the declaration being parsed, if any.
	// It's nil if the type expression isn't attached to a declaration.
	decl *Decl

	// typeParamsInScope contains the in-scope type parameters
	// as part of the declaration being processed.
	// It is nil if no declaration is being processed.
	//
	// The keys are the names of the type parameter ("T" and "U" in "type Foo[T any, U io.Reader]")
	// and the values are the index of the type parameter in the declaration (above, 0 for T and 1 for U).
	typeParamsInScope map[string]int
}

func (r *typeResolver) parseType(file *pkginfo.File, expr ast.Expr) Type {
	typ := func() Type {
		switch expr := expr.(type) {
		case *ast.StarExpr:
			// Pointer
			return PointerType{
				AST:  expr,
				Elem: r.parseType(file, expr.X),
			}

		case *ast.Ident:
			pkgNames := file.Pkg.Names()

			// Check if we have a type parameter defined for this
			if idx, ok := r.typeParamsInScope[expr.Name]; ok {
				return TypeParamRefType{
					AST:   expr,
					Decl:  r.decl,
					Index: idx,
				}
			}

			// Local type name or universe scope
			if d, ok := pkgNames.PkgDecls[expr.Name]; ok && d.Type == token.TYPE {
				decl := r.p.ParseDecl(d)
				return NamedType{
					AST:  expr,
					Decl: decl,
				}
			}

			// Finally check if it's a built-in type
			if b, ok := builtinTypes[expr.Name]; ok {
				if b == unsupported {
					r.errs.Addf(expr.Pos(), "unsupported type: %s", expr.Name)
				}
				return BuiltinType{
					AST:  expr,
					Kind: b,
				}
			}

			r.errs.Addf(expr.Pos(), "undefined type: %s", expr.Name)

		case *ast.SelectorExpr:
			fileNames := file.Names()

			// pkg.T
			if pkgName, ok := expr.X.(*ast.Ident); ok {
				pkgPath, ok := fileNames.ResolvePkgPath(pkgName.Name)
				if !ok {
					r.errs.Addf(expr.X.Pos(), "unknown package: %s", pkgName.Name)
					return nil
				}

				// Do we have a supported builtin?
				if kind, ok := r.parseEncoreBuiltin(pkgPath, expr.Sel.Name); ok {
					return BuiltinType{AST: expr, Kind: kind}
				}

				// Otherwise, load the external package and resolve the type.
				otherPkg := r.p.l.MustLoadPkg(pkgName.Pos(), pkgPath)
				if d, ok := otherPkg.Names().PkgDecls[expr.Sel.Name]; ok && d.Type == token.TYPE {
					decl := r.p.ParseDecl(d)
					return NamedType{AST: expr, Decl: decl}
				}
			}
			r.errs.Addf(expr.Pos(), "%s is not a type", types.ExprString(expr))

		case *ast.StructType:
			st := StructType{
				AST:    expr,
				Fields: make([]*StructField, 0, expr.Fields.NumFields()),
			}

			for _, field := range expr.Fields.List {
				typ := r.parseType(file, field.Type)
				if len(field.Names) == 0 {
					r.errs.Add(field.Pos(), "cannot use anonymous fields in Encore struct types")
				}

				for _, name := range field.Names {
					st.Fields = append(st.Fields, &StructField{
						AST:  field,
						Name: name.Name,
						Type: typ,
					})
				}
			}
			return st

		case *ast.MapType:
			key := r.parseType(file, expr.Key)
			value := r.parseType(file, expr.Value)
			return MapType{
				AST:   expr,
				Key:   key,
				Value: value,
			}

		case *ast.ArrayType:
			elem := r.parseType(file, expr.Elt)

			result := ListType{AST: expr, Len: -1, Elem: elem}
			if expr.Len == nil {
				// We have a slice, not an array.

				// Translate list of bytes to the builtin Bytes kind.
				if elem.Family() == Builtin && elem.(BuiltinType).Kind == Uint8 {
					return BuiltinType{AST: expr, Kind: Bytes}
				}
				return result
			}

			// We have an array. Determine its length.

			// It's possible to define arrays with length equal to some constant
			// expression, but we don't support parsing that right now and treat
			// it as a slice.
			if basicLit, ok := expr.Len.(*ast.BasicLit); ok && basicLit.Kind == token.INT {
				if n, err := strconv.Atoi(basicLit.Value); err == nil {
					result.Len = n
				}
			}

			// Fall back to a list type if we couldn't determine the length.
			return result

		case *ast.InterfaceType:
			r.errs.Add(expr.Pos(), "cannot use interface types in Encore schema definitions")

		case *ast.ChanType:
			r.errs.Add(expr.Pos(), "cannot use channel types in Encore schema definitions")

		case *ast.FuncType:
			r.errs.Add(expr.Pos(), "cannot use function types in Encore schema definitions")

		case *ast.IndexExpr:
			// Generic type application with a single type, like "Foo[int]"
			return r.resolveTypeWithTypeArgs(file, expr.X, []ast.Expr{expr.Index})

		case *ast.IndexListExpr:
			// Same as IndexExpr, but with multiple types, like "Foo[int, string]"
			return r.resolveTypeWithTypeArgs(file, expr.X, expr.Indices)

		default:
			r.errs.Addf(expr.Pos(), "%s is not a supported type; got %+v",
				types.ExprString(expr), reflect.TypeOf(expr))
		}
		r.errs.Bailout()
		return nil
	}()

	return typ
}

func (r *typeResolver) resolveTypeWithTypeArgs(file *pkginfo.File, expr ast.Expr, typeArgs []ast.Expr) Type {
	// TODO determine if it's correct to pass along the typeArgs here.
	// It was done in the old parser.
	baseType := r.parseType(file, expr)
	if baseType.Family() != Named {
		r.errs.Addf(expr.Pos(), "cannot use type arguments with non-named type %s", types.ExprString(baseType.ASTExpr()))
		return baseType
	}

	// TODO(andre) handle config types here

	named := baseType.(NamedType)
	decl := named.Decl
	if len(decl.TypeParams) != len(typeArgs) {
		r.errs.Addf(expr.Pos(), "expected %d type parameters, got %d for reference to %s",
			len(decl.TypeParams), len(typeArgs), decl.Name)
		return baseType
	}

	named.TypeArgs = make([]Type, len(typeArgs))
	for idx, expr := range typeArgs {
		named.TypeArgs[idx] = r.parseType(file, expr)
	}
	return named
}

const (
	uuidImportPath paths.Pkg = "encore.dev/types/uuid"
	authImportPath paths.Pkg = "encore.dev/beta/auth"
)

// ParseDecl parses the type from a package declaration.
func (p *Parser) ParseDecl(d *pkginfo.PkgDeclInfo) *Decl {
	pkg := d.File.Pkg
	// Have we already parsed this?
	key := declKey{pkg: pkg.ImportPath, name: d.Name}
	p.declsMu.Lock()
	decl, ok := p.decls[key]
	p.declsMu.Unlock()
	if ok {
		return decl
	}

	// We haven't parsed this yet; do so now.
	// Allocate a decl immediately so that we can properly handle
	// recursive types by short-circuiting above the second time we get here.
	spec, ok := d.Spec.(*ast.TypeSpec)
	if !ok {
		p.c.Errs.Fatal(d.Spec.Pos(), "unable to get TypeSpec from PkgDecl spec")
	}

	decl = &Decl{
		Name:       d.Name,
		Pkg:        pkg,
		TypeParams: nil,
		// Type is set below
	}
	p.declsMu.Lock()
	p.decls[key] = decl
	p.declsMu.Unlock()

	// If this is a parameterized declaration, get the type parameters
	var typeParamsInScope map[string]int
	if spec.TypeParams != nil {
		numParams := spec.TypeParams.NumFields()
		decl.TypeParams = make([]DeclTypeParam, 0, numParams)
		typeParamsInScope = make(map[string]int, numParams)

		paramIdx := 0
		for _, typeParam := range spec.TypeParams.List {
			for _, name := range typeParam.Names {
				decl.TypeParams = append(decl.TypeParams, DeclTypeParam{
					AST:  typeParam,
					Name: name.Name,
				})
				typeParamsInScope[name.Name] = paramIdx
				paramIdx++
			}
		}
	}

	r := p.newTypeResolver(decl, typeParamsInScope)
	decl.Type = r.parseType(d.File, spec.Type)

	return decl
}

// parseEncoreBuiltin returns the builtin kind for the given package path and name.
// If the type is not one of the Encore builtins it reports (unsupported, false).
func (r *typeResolver) parseEncoreBuiltin(pkgPath paths.Pkg, name string) (BuiltinKind, bool) {
	switch {
	case pkgPath == uuidImportPath && name == "UUID":
		return UUID, true
	case pkgPath == authImportPath && name == "UID":
		return UserID, true
	case pkgPath == "time" && name == "Time":
		return Time, true
	case pkgPath == "encoding/json" && name == "RawMessage":
		return JSON, true
	}
	return unsupported, false
}

var builtinTypes = map[string]BuiltinKind{
	"bool":       Bool,
	"int":        Int,
	"int8":       Int8,
	"int16":      Int16,
	"int32":      Int32,
	"int64":      Int64,
	"uint":       Uint,
	"uint8":      Uint8,
	"uint16":     Uint16,
	"uint32":     Uint32,
	"uint64":     Uint64,
	"uintptr":    unsupported,
	"float32":    Float32,
	"float64":    Float64,
	"complex64":  unsupported,
	"complex128": unsupported,
	"string":     String,
	"byte":       Uint8,
	"rune":       Uint32,
}

// declKey is a unique key for the given declaration.
type declKey struct {
	pkg  paths.Pkg
	name string
}
