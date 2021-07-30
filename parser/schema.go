package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	schema "encr.dev/proto/encore/parser/schema/v1"
	"github.com/fatih/structtag"
)

// resolveType parses the schema from a type expression.
func (p *parser) resolveDecl(pkg *est.Package, file *est.File, expr ast.Expr) *schema.Decl {
	typ := p.resolveType(pkg, file, expr)
	n := typ.GetNamed()
	if n == nil {
		p.errf(expr.Pos(), "%s is not a named type", types.ExprString(expr))
		panic(bailout{})
	}
	return p.decls[n.Id]
}

// resolveType parses the schema from a type expression.
func (p *parser) resolveType(pkg *est.Package, file *est.File, expr ast.Expr) *schema.Type {
	expr = deref(expr)
	names := p.names[pkg]

	switch expr := expr.(type) {
	case *ast.Ident:
		// Local type name or universe scope
		if d, ok := names.Decls[expr.Name]; ok && d.Type == token.TYPE {
			return p.parseDecl(pkg, d)
		} else if b, ok := builtinTypes[expr.Name]; ok {
			return &schema.Type{
				Typ: &schema.Type_Builtin{Builtin: b},
			}
		}
		p.errf(expr.Pos(), "undefined type: %s", expr.Name)

	case *ast.SelectorExpr:
		// pkg.T
		if pkgName, ok := expr.X.(*ast.Ident); ok {
			pkgPath := names.Files[file].NameToPath[pkgName.Name]
			if otherPkg, ok := p.pkgMap[pkgPath]; ok {
				if d, ok := p.names[otherPkg].Decls[expr.Sel.Name]; ok && d.Type == token.TYPE {
					return p.parseDecl(otherPkg, d)
				}
			} else {
				return p.parseEncoreBuiltin(expr.Pos(), pkgPath, expr.Sel.Name)
			}
		}
		p.errf(expr.Pos(), "%s is not a type", types.ExprString(expr))

	case *ast.StructType:
		st := &schema.Struct{}

		// Track seen names to make sure there aren't any name conflicts
		// in the presence of json tags.
		seenNames := make(map[string]token.Pos)
		checkName := func(pos token.Pos, name, typ string) {
			if pos2, ok := seenNames[name]; ok {
				pp := p.fset.Position(pos2)
				p.errf(pos, typ+" name %s conflicts with already defined name at %s", name, pp)
			} else {
				seenNames[name] = pos
			}
		}

		for _, field := range expr.Fields.List {
			typ := p.resolveType(pkg, file, field.Type)
			if len(field.Names) == 0 {
				p.err(field.Pos(), "cannot use anonymous fields in Encore struct types")
			}
			opts := p.parseStructTag(field.Tag)

			// Validate the names to make sure we don't have any name collisions
			if js := opts.JSONName; js != "" {
				if len(field.Names) > 1 {
					pp := p.fset.Position(field.Names[0].Pos())
					p.errf(field.Names[1].Pos(), "json field name %s conflicts with previous field (defined at %s)", js, pp)
				}
				checkName(field.Tag.Pos(), js, "json")
			} else {
				for _, name := range field.Names {
					checkName(name.Pos(), name.Name, "field")
				}
			}

			for _, name := range field.Names {
				f := &schema.Field{
					Typ:      typ,
					Name:     name.Name,
					Doc:      field.Doc.Text(),
					Optional: opts.Optional,
					JsonName: opts.JSONName,
				}
				st.Fields = append(st.Fields, f)
			}
		}
		return &schema.Type{Typ: &schema.Type_Struct{Struct: st}}

	case *ast.MapType:
		key := p.resolveType(pkg, file, expr.Key)
		value := p.resolveType(pkg, file, expr.Value)
		return &schema.Type{Typ: &schema.Type_Map{Map: &schema.Map{Key: key, Value: value}}}

	case *ast.ArrayType:
		elem := p.resolveType(pkg, file, expr.Elt)
		// Translate []byte to BYTES
		if b, ok := elem.Typ.(*schema.Type_Builtin); ok && b.Builtin == schema.Builtin_UINT8 {
			return &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_BYTES}}
		}
		return &schema.Type{Typ: &schema.Type_List{List: &schema.List{Elem: elem}}}

	case *ast.InterfaceType:
		p.err(expr.Pos(), "cannot use interface types in Encore schema definitions")

	case *ast.ChanType:
		p.err(expr.Pos(), "cannot use channel types in Encore schema definitions")

	case *ast.FuncType:
		p.err(expr.Pos(), "cannot use function types in Encore schema definitions")

	default:
		p.errf(expr.Pos(), "%s is not a type", types.ExprString(expr))
	}

	panic(bailout{})
}

// parseDecl parses the type from a package declaration.
func (p *parser) parseDecl(pkg *est.Package, d *names.PkgDecl) *schema.Type {
	key := pkg.ImportPath + "." + d.Name
	decl, ok := p.declMap[key]
	if !ok {
		// We haven't parsed this yet; do so now.
		// Allocate a decl immediately so that we can properly handle
		// recursive types by short-circuiting above the second time we get here.
		id := uint32(len(p.decls))
		typ := d.Spec.(*ast.TypeSpec).Type
		decl = &schema.Decl{
			Id:   id,
			Name: d.Name,
			Doc:  d.Doc,
			Loc:  parseLoc(d.File, typ),
			// Type is set below
		}
		p.declMap[key] = decl
		p.decls = append(p.decls, decl)
		decl.Type = p.resolveType(pkg, d.File, d.Spec.(*ast.TypeSpec).Type)
	}

	return &schema.Type{Typ: &schema.Type_Named{
		Named: &schema.Named{Id: decl.Id},
	}}
}

func (p *parser) parseEncoreBuiltin(pos token.Pos, pkgPath, name string) *schema.Type {
	switch {
	case pkgPath == uuidImportPath && name == "UUID":
		return &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_UUID}}
	case pkgPath == authImportPath && name == "UID":
		return &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_USER_ID}}
	case pkgPath == "time" && name == "Time":
		return &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_TIME}}
	case pkgPath == "encoding/json" && name == "RawMessage":
		return &schema.Type{Typ: &schema.Type_Builtin{Builtin: schema.Builtin_JSON}}
	}
	p.errf(pos, "%s.%s is not a supported type in Encore\n\tnote: you can only use types defined within your Encore app and builtins\n\tbuiltins also include time.Time, json.RawMessage, and encore.dev/types/uuid.UUID", pkgPath, name)
	panic(bailout{})
}

// structFieldOptions represents the parsed struct tag information
// that Encore recognizes.
type structFieldOptions struct {
	// JSONName is set if there is a distinct json name (`json:"foo"`).
	// If JSONName == "-" it indicates to omit the field entirely.
	JSONName string
	// Optional is true if there is an `encore:"optional"` tag
	Optional bool
}

// parseStructTag parses the struct tag to determine any encore-specific options
// and the JSON name, if any.
func (p *parser) parseStructTag(tag *ast.BasicLit) structFieldOptions {
	var opts structFieldOptions
	if tag == nil {
		return opts
	}

	val, _ := strconv.Unquote(tag.Value)
	tags, err := structtag.Parse(val)
	if err != nil {
		p.errf(tag.Pos(), "invalid struct tag: %v", err)
		return opts
	}
	if enc, _ := tags.Get("encore"); enc != nil {
		ops := append([]string{enc.Name}, enc.Options...)
		for _, o := range ops {
			switch o {
			case "optional":
				opts.Optional = true
			default:
				p.errf(tag.Pos(), "invalid encore struct tag option: %s", o)
			}
		}
	}
	if js, _ := tags.Get("json"); js != nil {
		if v := js.Value(); v != "" {
			opts.JSONName = v
		}
	}
	return opts
}

func getField(list *ast.FieldList, n int) (field *ast.Field, name string) {
	i := 0
	for _, f := range list.List {
		num := len(f.Names)
		if num == 0 {
			num = 1
		}
		idx := n - i
		if idx < num {
			if len(f.Names) == 0 {
				return f, ""
			}
			return f, f.Names[idx].Name
		}
		i += num
	}
	return nil, ""
}

func deref(expr ast.Expr) ast.Expr {
	for {
		star, ok := expr.(*ast.StarExpr)
		if !ok {
			break
		}
		expr = star.X
	}
	return expr
}

var builtinTypes = map[string]schema.Builtin{
	"bool":       schema.Builtin_BOOL,
	"int":        schema.Builtin_INT,
	"int8":       schema.Builtin_INT8,
	"int16":      schema.Builtin_INT16,
	"int32":      schema.Builtin_INT32,
	"int64":      schema.Builtin_INT64,
	"uint":       schema.Builtin_UINT,
	"uint8":      schema.Builtin_UINT8,
	"uint16":     schema.Builtin_UINT16,
	"uint32":     schema.Builtin_UINT32,
	"uint64":     schema.Builtin_UINT64,
	"uintptr":    -1,
	"float32":    schema.Builtin_FLOAT32,
	"float64":    schema.Builtin_FLOAT64,
	"complex64":  -1,
	"complex128": -1,
	"string":     schema.Builtin_STRING,
	"byte":       schema.Builtin_UINT8,
	"rune":       schema.Builtin_UINT32,
}
