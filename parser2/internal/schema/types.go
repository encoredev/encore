package schema

import (
	"go/ast"
	"sync"

	"encr.dev/parser2/internal/pkginfo"
)

// Decl represents a type declaration.
type Decl struct {
	// AST is the AST node that this declaration represents.
	AST ast.Decl

	Pkg  *pkginfo.Package // the package declaring the type
	Name string           // name of the type declaration
	Type Type             // the declaration's underlying type

	// TypeParams are any type parameters on this declaration (note; instantiated types used within this declaration would not be captured here)
	TypeParams []DeclTypeParam
}

// DeclTypeParam represents a type parameter on a declaration.
// For example A in "type Foo[A any] struct { ... }"
type DeclTypeParam struct {
	// AST is the AST node that this type param represents.
	// Note that multiple fields may share the same *ast.Field node,
	// in cases with multiple names, like "type Foo[A, B any]".
	AST *ast.Field

	Name string // the identifier given to the type parameter.
}

type TypeFamily int

const (
	Invalid TypeFamily = iota
	Named
	Struct
	Map
	List
	Builtin
	Pointer
	TypeParamRef
)

type Type interface {
	Family() TypeFamily
	ASTExpr() ast.Expr
}

type NamedType struct {
	AST ast.Expr // *ast.Ident or *ast.SelectorExpr

	// Decl is the declaration this type refers to.
	Decl *Decl

	// TypeArgs are the type arguments used to instantiate this named type.
	TypeArgs []Type
}

type StructType struct {
	AST    *ast.StructType
	Fields []*StructField
}

type StructField struct {
	// AST is the AST node that this field represents.
	// Note that multiple fields may share the same *ast.Field node,
	// in cases with multiple names, like "Foo, Bar int".
	AST *ast.Field

	Name string
	Type Type

	docOnce   sync.Once
	cachedDoc string
}

func (f *StructField) Anonymous() bool {
	return f.Name == ""
}

func (f *StructField) Exported() bool {
	return f.Name == "" || ast.IsExported(f.Name)
}

// Doc returns the doc comment for the field, if any.
func (f *StructField) Doc() string {
	f.docOnce.Do(func() {
		// Use the documentation block above the field by default,
		// however if that is blank, then use the line comment instead
		docBlock := f.AST.Doc
		if docBlock == nil || docBlock.Text() == "" {
			docBlock = f.AST.Comment
		}
		f.cachedDoc = docBlock.Text()
	})
	return f.cachedDoc
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

type BuiltinType struct {
	AST  ast.Expr
	Kind BuiltinKind
}

// TypeParamRefType is a reference to a `TypeParameter` within a declaration block
type TypeParamRefType struct {
	AST *ast.Ident

	Decl  *Decl
	Index int // Index into the type parameter slice on the declaration
}

// TODO(andre) Config Values

type BuiltinKind int

const (
	Any BuiltinKind = iota
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
var _ Type = BuiltinType{}
var _ Type = TypeParamRefType{}

func (NamedType) Family() TypeFamily        { return Named }
func (StructType) Family() TypeFamily       { return Struct }
func (MapType) Family() TypeFamily          { return Map }
func (ListType) Family() TypeFamily         { return List }
func (PointerType) Family() TypeFamily      { return Pointer }
func (BuiltinType) Family() TypeFamily      { return Builtin }
func (TypeParamRefType) Family() TypeFamily { return TypeParamRef }

func (t NamedType) ASTExpr() ast.Expr        { return t.AST }
func (t StructType) ASTExpr() ast.Expr       { return t.AST }
func (t MapType) ASTExpr() ast.Expr          { return t.AST }
func (t ListType) ASTExpr() ast.Expr         { return t.AST }
func (t PointerType) ASTExpr() ast.Expr      { return t.AST }
func (t BuiltinType) ASTExpr() ast.Expr      { return t.AST }
func (t TypeParamRefType) ASTExpr() ast.Expr { return t.AST }
