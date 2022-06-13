package parser

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/fatih/structtag"

	"encr.dev/parser/est"
	"encr.dev/pkg/errlist"
	"encr.dev/pkg/identifiers"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

var additionalTypeResolver = func(p *parser, pkg *est.Package, file *est.File, expr ast.Expr) *schema.Type { return nil }

// disallowedHeaders is a list of headers that we will not let an
// application either read from requests or write in responses.
var disallowedHeaders = []string{
	"Cookie", "Cookie2", "Set-Cookie", "Set-Cookie2",
	"Upgrade",
}

// resolveType parses the schema from a type expression.
func (p *parser) resolveType(pkg *est.Package, file *est.File, expr ast.Expr, typeParameters typeParameterLookup) *schema.Type {
	expr = deref(expr)
	pkgNames := p.names[pkg]

	switch expr := expr.(type) {
	case *ast.Ident:
		// Check if we have a type parameter defined for this
		if ref, ok := typeParameters[expr.Name]; ok {
			return &schema.Type{Typ: &schema.Type_TypeParameter{TypeParameter: ref}}
		}

		// Local type name or universe scope
		if d, ok := pkgNames.Decls[expr.Name]; ok && d.Type == token.TYPE {
			return p.parseDecl(pkg, d, typeParameters)
		}

		// Finally check if it's a built-in type
		if b, ok := builtinTypes[expr.Name]; ok {
			return &schema.Type{
				Typ: &schema.Type_Builtin{Builtin: b},
			}
		}

		p.errf(expr.Pos(), "undefined type: %s", expr.Name)

	case *ast.SelectorExpr:
		// pkg.T
		if pkgName, ok := expr.X.(*ast.Ident); ok {
			pkgPath := pkgNames.Files[file].NameToPath[pkgName.Name]
			if otherPkg, ok := p.pkgMap[pkgPath]; ok {
				if d, ok := p.names[otherPkg].Decls[expr.Sel.Name]; ok && d.Type == token.TYPE {
					return p.parseDecl(otherPkg, d, typeParameters)
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
			typ := p.resolveType(pkg, file, field.Type, typeParameters)
			if len(field.Names) == 0 {
				p.err(field.Pos(), "cannot use anonymous fields in Encore struct types")
			}
			opts := p.parseStructTag(field.Tag, typ)

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
				// Skip unexported fields
				if !ast.IsExported(name.Name) {
					continue
				}
				// Use the documentation block above the field by default,
				// however if that is blank, then use the line comment instead
				docBlock := field.Doc
				if docBlock == nil || docBlock.Text() == "" {
					docBlock = field.Comment
				}

				f := &schema.Field{
					Typ:             typ,
					Name:            name.Name,
					Doc:             docBlock.Text(),
					Optional:        opts.Optional,
					JsonName:        opts.JSONName,
					QueryStringName: opts.QueryStringName,
					Tags:            schemaTags(opts.Tags),
					RawTag:          opts.RawTag,
				}
				if f.QueryStringName == "" {
					f.QueryStringName = identifiers.ConvertIdentifierTo(f.Name, identifiers.SnakeCase)
				}

				st.Fields = append(st.Fields, f)
			}
		}
		return &schema.Type{Typ: &schema.Type_Struct{Struct: st}}

	case *ast.MapType:
		key := p.resolveType(pkg, file, expr.Key, typeParameters)
		value := p.resolveType(pkg, file, expr.Value, typeParameters)
		return &schema.Type{Typ: &schema.Type_Map{Map: &schema.Map{Key: key, Value: value}}}

	case *ast.ArrayType:
		elem := p.resolveType(pkg, file, expr.Elt, typeParameters)
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
		if resolvedType := additionalTypeResolver(p, pkg, file, expr); resolvedType != nil {
			return resolvedType
		}

		p.errf(expr.Pos(), "%s is not a supported type; got %+v", types.ExprString(expr), reflect.TypeOf(expr))
	}

	panic(errlist.Bailout{})
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
	panic(errlist.Bailout{})
}

// structFieldOptions represents the parsed struct tag information
// that Encore recognizes.
type structFieldOptions struct {
	// JSONName is set if there is a distinct json name (`json:"foo"`).
	// If JSONName == "-" it indicates to omit the field entirely.
	JSONName string
	// QueryStringName is set if there is a distinct query string name (`qs:"foo"`).
	// If QueryStringName == "-" it indicates to omit the field entirely.
	QueryStringName string
	// Optional is true if there is an `encore:"optional"` tag
	Optional bool
	// Tags contains parsed struct field tags
	Tags []*structtag.Tag
	// RawTag contains the raw unparsed struct field tag (if any)
	RawTag string
}

// schemaTags converts structtag.Tags to an array of proto schema.Tag
func schemaTags(tags []*structtag.Tag) []*schema.Tag {
	rtn := make([]*schema.Tag, len(tags))
	for i, t := range tags {
		rtn[i] = &schema.Tag{
			Key:     t.Key,
			Name:    t.Name,
			Options: t.Options,
		}
	}
	return rtn
}

// parseStructTag parses the struct tag to determine any encore-specific options
// and the JSON name, if any.
func (p *parser) parseStructTag(tag *ast.BasicLit, resolvedType *schema.Type) structFieldOptions {
	var opts structFieldOptions
	if tag == nil {
		return opts
	}

	val, _ := strconv.Unquote(tag.Value)
	opts.RawTag = val
	tags, err := structtag.Parse(val)
	if err != nil {
		p.errf(tag.Pos(), "invalid struct tag: %v", err)
		return opts
	}
	opts.Tags = tags.Tags()

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
		if v := js.Name; v != "" {
			opts.JSONName = v
		}
	}
	if qs, _ := tags.Get("qs"); qs != nil {
		if v := qs.Name; v != "" {
			opts.QueryStringName = v
		}
	}

	if header, _ := tags.Get("header"); header != nil {
		// Due to way headers are encoded, the RFC specifies that multiple values for the same
		// header should be encoded as a comma-separated list.
		//
		// This would lead to undefined behaviour if we allowed string slices, while Go would
		// encode each value as a separate Header line, some clients (Javascript) would combine
		// these into one comma seperated header - which would then need to split on commas to get
		// the original slices.
		//
		// This results in the slice from the server of { "foo, bar", "zar" } either being received
		// as the list { "foo", "bar", "zar" } or { "foo, bar", "zar" } depending on the client
		// implementation.
		//
		// We've decided against doing any encoding of strings, as that would be unexpected and hidden
		// behaviour from handwritten clients using the APIs.
		if resolvedType.GetList() != nil {
			p.errf(tag.Pos(), "header tags are not allowed on slices")
		}

		// We're not allowing generic fields in headers, as at parse time we cannot know if the generic
		// is being used as a slice.
		if resolvedType.GetTypeParameter() != nil {
			p.errf(tag.Pos(), "header tags are not allowed on generic fields")
		}

		// We don't allow certain header name to be set for security reasons
		for _, headerName := range disallowedHeaders {
			if strings.EqualFold(headerName, strings.TrimSpace(header.Name)) {
				p.errf(tag.Pos(), "a header name of %s is not allowed", headerName)
			}
		}

		// Because we need to do type casting in the clients, we're limiting the types to built in Encore types
		switch resolvedType.Typ.(type) {
		case *schema.Type_Builtin:
			// no-op
		default:
			p.errf(tag.Pos(), "header tags can only be used on built in types or types provided by Encore")
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
