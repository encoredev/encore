package schema

import (
	"go/ast"
	"go/types"
	"sync"

	"github.com/fatih/structtag"

	"encr.dev/pkg/option"
	"encr.dev/v2/internals/pkginfo"
)

type TypeFamily int

const (
	Unknown TypeFamily = iota
	Named
	Struct
	Map
	List
	Builtin
	Pointer
	Option
	Func
	Interface
	TypeParamRef
)

type Type interface {
	Family() TypeFamily
	ASTExpr() ast.Expr
	String() string // Resolve to a string representation of the type.
}

type NamedType struct {
	AST ast.Expr // *ast.Ident or *ast.SelectorExpr

	// DeclInfo is the declaration info for the declaration
	// this refers to.
	DeclInfo *pkginfo.PkgDeclInfo

	// TypeArgs are the type arguments used to instantiate this named type.
	TypeArgs []Type

	// decl lazy-initializes the type declaration.
	// It's a pointer since it's stateful via sync.Once.
	decl *lazyDecl
}

// lazyDecl is a lazily-initialized type decl.
type lazyDecl struct {
	p    *Parser
	info *pkginfo.PkgDeclInfo

	once sync.Once
	decl *TypeDecl
}

func (t NamedType) Decl() *TypeDecl {
	return t.decl.Decl()
}

func (t NamedType) WithDecl(decl *TypeDecl) NamedType {
	return newEagerNamedType(t.AST, t.TypeArgs, decl)
}

func (d *lazyDecl) Decl() *TypeDecl {
	d.once.Do(func() {
		d.decl = d.p.ParseTypeDecl(d.info)
	})
	return d.decl
}

type StructType struct {
	AST    *ast.StructType
	Fields []StructField
}

type StructField struct {
	// AST is the AST node that this field represents.
	// Note that multiple fields may share the same *ast.Field node,
	// in cases with multiple names, like "Foo, Bar int".
	AST *ast.Field

	Name option.Option[string] // field name, or None if anonymous
	Type Type
	Doc  string
	Tag  structtag.Tags
}

func (f *StructField) IsAnonymous() bool {
	return f.Name.Empty()
}

func (f *StructField) IsExported() bool {
	return f.IsAnonymous() || f.Name.Contains(ast.IsExported)
}

type MapType struct {
	AST   *ast.MapType
	Key   Type
	Value Type
}

type ListType struct {
	AST  *ast.ArrayType
	Len  int // -1 for a slice
	Elem Type
}

type PointerType struct {
	AST  *ast.StarExpr
	Elem Type
}

// OptionType represents an option.Option[T] type.
type OptionType struct {
	AST   ast.Expr
	Value Type
}

type BuiltinType struct {
	AST  ast.Expr
	Kind BuiltinKind
}

type FuncType struct {
	AST     *ast.FuncType
	Params  []Param
	Results []Param
}

type InterfaceType struct {
	AST *ast.InterfaceType

	// EmbeddedIfaces are the interfaces this interface embeds.
	EmbeddedIfaces []Type

	// TODO change these out for more useful information.
	TypeLists []ast.Expr
	Methods   []*ast.Field
}

// Param represents a parameter or result field.
type Param struct {
	AST  *ast.Field
	Name option.Option[string] // parameter name, or None if a type-only parameter.
	Type Type
}

// TypeParamRefType is a reference to a `TypeParameter` within a declaration block
type TypeParamRefType struct {
	AST *ast.Ident

	Decl  Decl // the declaration this type parameter is defined on
	Index int  // Index into the type parameter slice on the declaration
}

type BuiltinKind int

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=BuiltinKind -output=types_string.go

const (
	Invalid BuiltinKind = iota
	Any
	Bool

	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64

	Float32
	Float64
	String
	Bytes

	// Additional Encore Types

	Time
	UUID
	JSON
	UserID
	Error // builtin "error" type, for convenience

	// unsupported is a special value used
	// to indicate the particular builtin is known,
	// but is not something Encore supports.
	unsupported BuiltinKind = -1
)

var _ Type = NamedType{}
var _ Type = StructType{}
var _ Type = MapType{}
var _ Type = ListType{}
var _ Type = PointerType{}
var _ Type = OptionType{}
var _ Type = BuiltinType{}
var _ Type = FuncType{}
var _ Type = InterfaceType{}
var _ Type = TypeParamRefType{}

func (NamedType) Family() TypeFamily        { return Named }
func (StructType) Family() TypeFamily       { return Struct }
func (MapType) Family() TypeFamily          { return Map }
func (ListType) Family() TypeFamily         { return List }
func (PointerType) Family() TypeFamily      { return Pointer }
func (OptionType) Family() TypeFamily       { return Option }
func (BuiltinType) Family() TypeFamily      { return Builtin }
func (FuncType) Family() TypeFamily         { return Func }
func (InterfaceType) Family() TypeFamily    { return Interface }
func (TypeParamRefType) Family() TypeFamily { return TypeParamRef }

func (t NamedType) ASTExpr() ast.Expr        { return t.AST }
func (t StructType) ASTExpr() ast.Expr       { return t.AST }
func (t MapType) ASTExpr() ast.Expr          { return t.AST }
func (t ListType) ASTExpr() ast.Expr         { return t.AST }
func (t PointerType) ASTExpr() ast.Expr      { return t.AST }
func (t OptionType) ASTExpr() ast.Expr       { return t.AST }
func (t BuiltinType) ASTExpr() ast.Expr      { return t.AST }
func (t FuncType) ASTExpr() ast.Expr         { return t.AST }
func (t InterfaceType) ASTExpr() ast.Expr    { return t.AST }
func (t TypeParamRefType) ASTExpr() ast.Expr { return t.AST }

func (t NamedType) String() string {
	name := t.DeclInfo.Name
	if t.TypeArgs != nil {
		name += "["
		for i, arg := range t.TypeArgs {
			if i > 0 {
				name += ", "
			}
			name += arg.String()
		}
		name += "]"
	}
	return name
}
func (t StructType) String() string       { return "struct" }
func (t MapType) String() string          { return "map[" + t.Key.String() + "]" + t.Value.String() }
func (t ListType) String() string         { return "[]" + t.Elem.String() }
func (t PointerType) String() string      { return "*" + t.Elem.String() }
func (t OptionType) String() string       { return "Option[" + t.Value.String() + "]" }
func (t BuiltinType) String() string      { return types.ExprString(t.AST) }
func (t FuncType) String() string         { return "function" }
func (t InterfaceType) String() string    { return "interface" }
func (t TypeParamRefType) String() string { return t.AST.Name }

// TypeDeclRef is a reference to a type declaration, through zero or more pointers
// and possibly with type arguments.
type TypeDeclRef struct {
	Decl     *TypeDecl
	TypeArgs []Type
	Pointers int
}

func (r *TypeDeclRef) ToType() Type {
	var typ Type = newEagerNamedType(r.Decl.Type.ASTExpr(), r.TypeArgs, r.Decl)
	for i := 0; i < r.Pointers; i++ {
		typ = PointerType{Elem: typ}
	}
	return typ
}
