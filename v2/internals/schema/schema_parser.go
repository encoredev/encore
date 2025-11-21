// Package schema implements parsing of Go types into Encore's schema format.
package schema

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"sync"

	"github.com/fatih/structtag"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
)

// NewParser constructs a new schema parser.
func NewParser(c *parsectx.Context, l *pkginfo.Loader) *Parser {
	return &Parser{
		c:     c,
		l:     l,
		decls: make(map[declKey]Decl),
	}
}

// Parser parses Go types into Encore's schema format.
type Parser struct {
	c *parsectx.Context
	l *pkginfo.Loader

	declsMu sync.Mutex
	decls   map[declKey]Decl // pkg/path.Name -> decl
}

// ParseType parses the schema from a type expression.
func (p *Parser) ParseType(file *pkginfo.File, expr ast.Expr) Type {
	r := p.newTypeResolver(nil, nil)
	return r.parseType(file, expr)
}

// newTypeResolver is a helper function to create a new typeResolver.
func (p *Parser) newTypeResolver(decl Decl, typeParamsInScope map[string]int) *typeResolver {
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
	decl Decl

	// typeParamsInScope contains the in-scope type parameters
	// as part of the declaration being processed.
	// It is nil if no declaration is being processed.
	//
	// The keys are the names of the type parameter ("T" and "U" in "type Foo[T any, U io.Reader]")
	// and the values are the index of the type parameter in the declaration (above, 0 for T and 1 for U).
	typeParamsInScope map[string]int
}

// parseType parses a type expression and returns it.
//
// This function will never return nil as it will Bailout upon any error.
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

			// Local type name
			if d, ok := pkgNames.PkgDecls[expr.Name]; ok && d.Type == token.TYPE {
				return newNamedType(r.p, expr, d)
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

			switch expr.Name {
			case "any":
				return InterfaceType{
					AST: &ast.InterfaceType{
						// HACK: Set dummy positions to make the error messages nicer,
						// pointing at "any" instead of reporting no position whatsoever.
						Interface: expr.Pos(),
						Methods: &ast.FieldList{
							Opening: expr.Pos(),
							Closing: expr.End() - 1,
						},
					},
				}
			}

			r.errs.Addf(expr.Pos(), "undefined type: %s", expr.Name)

		case *ast.SelectorExpr:
			fileNames := file.Names()

			// pkg.T
			if pkgName, ok := expr.X.(*ast.Ident); ok {
				pkgPath, ok := fileNames.ResolvePkgPath(pkgName.Pos(), pkgName.Name)
				if !ok {
					r.errs.Addf(expr.X.Pos(), "unknown package: %s", pkgName.Name)
					r.errs.Bailout()
				}

				// Do we have a supported builtin?
				if kind, ok := r.parseEncoreBuiltin(pkgPath, expr.Sel.Name); ok {
					return BuiltinType{AST: expr, Kind: kind}
				}

				// Otherwise, load the external package and resolve the type.
				otherPkg := r.p.l.MustLoadPkg(pkgName.Pos(), pkgPath)
				if d, ok := otherPkg.Names().PkgDecls[expr.Sel.Name]; ok && d.Type == token.TYPE {
					return newNamedType(r.p, expr, d)
				}
			}
			r.errs.Addf(expr.Pos(), "%s is not a type", types.ExprString(expr))

		case *ast.StructType:
			st := StructType{
				AST:    expr,
				Fields: make([]StructField, 0, expr.Fields.NumFields()),
			}

			for _, field := range expr.Fields.List {
				typ := r.parseType(file, field.Type)
				if len(field.Names) == 0 {
					// r.errs.AddPos(field.Pos(), "cannot use anonymous fields in Encore struct types")
					continue
				}

				// Parse the struct tags, if any.
				var tags structtag.Tags
				if field.Tag != nil {
					val, _ := strconv.Unquote(field.Tag.Value)
					t, err := structtag.Parse(val)
					if err != nil {
						r.errs.Addf(field.Tag.Pos(), "invalid struct tag: %v", err.Error())
					} else {
						tags = *t
					}
				}

				docs := field.Doc.Text()
				if docs == "" {
					docs = field.Comment.Text()
				}

				for _, name := range field.Names {
					st.Fields = append(st.Fields, StructField{
						AST:  field,
						Name: option.Some(name.Name),
						Type: typ,
						Tag:  tags,
						Doc:  docs,
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
			// TODO(andre) Parse more complete information about the interface.
			typ := InterfaceType{AST: expr}

			if expr.Methods != nil {
				for _, field := range expr.Methods.List {
					switch {
					case len(field.Names) > 0:
						typ.Methods = append(typ.Methods, field)
					case field.Type == nil:
						// shouldn't happen but let's be defensive
					default:
						// type switch or embedded interface
						switch field.Type.(type) {
						case *ast.BinaryExpr, *ast.UnaryExpr:
							typ.TypeLists = append(typ.TypeLists, field.Type)
						default:
							t := r.parseType(file, field.Type)
							typ.EmbeddedIfaces = append(typ.EmbeddedIfaces, t)
						}
					}
				}
			}

			return typ

		case *ast.ChanType:
			r.errs.AddPos(expr.Pos(), "cannot use channel types in Encore schema definitions")

		case *ast.FuncType:
			return r.parseFuncType(file, expr)

		case *ast.IndexExpr:
			// Generic type application with a single type, like "Foo[int]"
			return r.resolveTypeWithTypeArgs(file, expr, expr.X, []ast.Expr{expr.Index})

		case *ast.IndexListExpr:
			// Same as IndexExpr, but with multiple types, like "Foo[int, string]"
			return r.resolveTypeWithTypeArgs(file, expr, expr.X, expr.Indices)

		default:
			r.errs.Addf(expr.Pos(), "%s is not a supported type; got %+v",
				types.ExprString(expr), reflect.TypeOf(expr))
		}
		r.errs.Bailout()
		return nil
	}()

	return typ
}

// parseFuncType parses an *ast.FuncType into a *FuncType.
func (r *typeResolver) parseFuncType(file *pkginfo.File, ft *ast.FuncType) FuncType {
	res := FuncType{
		AST:     ft,
		Params:  make([]Param, 0, ft.Params.NumFields()),
		Results: make([]Param, 0, ft.Results.NumFields()),
	}

	// iters describes how to iterate over the parameters and results.
	iters := []struct {
		fields *ast.FieldList
		dst    *[]Param
	}{
		{ft.Params, &res.Params},
		{ft.Results, &res.Results},
	}

	// Loop over all the fields and parse the types.
	for _, it := range iters {
		// The fields are nil if the function has no parameters or results.
		if it.fields == nil {
			continue
		}

		for _, field := range it.fields.List {
			typ := r.parseType(file, field.Type)
			// If we have any names it means they all have names.
			if len(field.Names) > 0 {
				for _, name := range field.Names {
					*it.dst = append(*it.dst, Param{
						AST:  field,
						Name: option.Some(name.Name),
						Type: typ,
					})
				}
			} else {
				// Otherwise we have a type-only parameter.
				*it.dst = append(*it.dst, Param{
					AST:  field,
					Name: option.None[string](),
					Type: typ,
				})
			}
		}
	}

	return res
}

func (r *typeResolver) resolveTypeWithTypeArgs(file *pkginfo.File, indexExpr, expr ast.Expr, typeArgs []ast.Expr) Type {
	baseType := r.parseType(file, expr)
	if baseType.Family() != Named {
		r.errs.Addf(expr.Pos(), "cannot use type arguments with non-named type %s", types.ExprString(baseType.ASTExpr()))
		return baseType
	}

	named := baseType.(NamedType)

	named.AST = indexExpr // Use the index expression as the AST for the named type as we've resolved the type arguments.
	decl := named.Decl()
	if len(decl.TypeParams) != len(typeArgs) {
		r.errs.Addf(expr.Pos(), "expected %d type parameters, got %d for reference to %s",
			len(decl.TypeParams), len(typeArgs), decl.Name)
		return baseType
	}

	named.TypeArgs = make([]Type, len(typeArgs))
	for idx, expr := range typeArgs {
		named.TypeArgs[idx] = r.parseType(file, expr)
	}

	// Is this an option.Option type? If so, we return an OptionType instead.
	if info := named.DeclInfo.QualifiedName(); info.PkgPath == optionImportPath && info.Name == "Option" {
		if len(named.TypeArgs) != 1 {
			r.errs.Addf(expr.Pos(), "option.Option must have exactly one type argument, got %d", len(named.TypeArgs))
			return named
		}
		return OptionType{AST: expr, Value: named.TypeArgs[0]}
	}

	return named
}

const (
	uuidImportPath   paths.Pkg = "encore.dev/types/uuid"
	optionImportPath paths.Pkg = "encore.dev/types/option"
	authImportPath   paths.Pkg = "encore.dev/beta/auth"
)

// parseRecv parses a receiver AST into a Receiver.
func (p *Parser) parseRecv(f *pkginfo.File, fields *ast.FieldList) (*Receiver, bool) {
	if fields.NumFields() != 1 {
		p.c.Errs.Add(errExpectedOnReciever(fields.NumFields()).AtGoNode(fields))
		return nil, false
	}

	// To properly parse the receiver in the presence of type parameters,
	// we first need to resolve what the named type is that the receiver is attached to
	// WITHOUT the type parameters. Then we can construct a typeResolver
	// with the correct decl context.
	field := fields.List[0]
	recvIdent := p.resolveReceiverIdent(field.Type)
	pkgDecl := f.Pkg.Names().PkgDecls[recvIdent.Name]
	if pkgDecl == nil {
		p.c.Errs.Add(errUnknownIdentifier(recvIdent.Name).AtGoNode(recvIdent))
		return nil, false
	}
	decl := p.ParseTypeDecl(pkgDecl)

	// Now that we have the declaration we can create a type resolver that
	// uses the type declaration's type parameters, and use that to resolve
	// the receiver type.
	typeParamsInScope, _ := computeDeclTypeParams(decl.AST.TypeParams)
	tr := p.newTypeResolver(decl, typeParamsInScope)

	recv := &Receiver{
		AST:  fields,
		Decl: decl,
		Type: tr.parseType(f, field.Type),
	}
	if len(field.Names) > 0 {
		recv.Name = option.Some(field.Names[0].Name)
	}

	return recv, true
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
	"error":      Error,
}

// resolveReceiverIdent resolves the identifier for the receiver of a method.
// It recurses through *ast.StarExpr, *ast.IndexExpr and *ast.IndexListExpr
// to handle pointer/non-pointer as well as methods on generic types.
func (p *Parser) resolveReceiverIdent(expr ast.Expr) *ast.Ident {
	orig := expr // keep track of original for error messages
	for i := 0; i < 10; i++ {
		switch x := expr.(type) {
		case *ast.Ident:
			// We're done
			return x

		case *ast.StarExpr:
			expr = x.X
		case *ast.IndexExpr:
			expr = x.X
		case *ast.IndexListExpr:
			expr = x.X
		default:
			p.c.Errs.Addf(orig.Pos(), "invalid receiver expression: %s (invalid type %T)", types.ExprString(orig), x)
		}
	}
	p.c.Errs.Fatalf(orig.Pos(), "invalid receiver expression: %s (recursion limit reached)", types.ExprString(orig))
	return nil // unreachable
}

// newNamedType is a helper to construct a lazy-loaded NamedType.
func newNamedType(p *Parser, expr ast.Expr, info *pkginfo.PkgDeclInfo) NamedType {
	return NamedType{
		AST:      expr,
		DeclInfo: info,
		decl: &lazyDecl{
			p:    p,
			info: info,
		},
	}
}

// newEagerNamedType is a helper to construct an eagerly loaded NamedType.
func newEagerNamedType(expr ast.Expr, typeArgs []Type, decl *TypeDecl) NamedType {
	lazy := &lazyDecl{}
	lazy.once.Do(func() {}) // mark the lazy.once as used
	lazy.decl = decl

	return NamedType{
		AST:      expr,
		DeclInfo: decl.Info,
		TypeArgs: typeArgs,
		decl:     lazy,
	}
}
