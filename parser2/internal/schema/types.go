package schema

import (
	"fmt"
	"go/ast"
	"sync"

	"github.com/cockroachdb/errors"
)

type TypeFamily int

const (
	Invalid TypeFamily = iota
	Named
	Struct
	Map
	List
	Builtin
	Pointer
	Func
	TypeParamRef
)

type Type interface {
	Family() TypeFamily
	ASTExpr() ast.Expr
}

type NamedType struct {
	AST ast.Expr // *ast.Ident or *ast.SelectorExpr

	// Decl is the declaration this type refers to.
	Decl *TypeDecl

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

	Name Optional[string] // field name, or None if anonymous
	Type Type

	docOnce   sync.Once
	cachedDoc string
}

func (f *StructField) IsAnonymous() bool {
	return !f.Name.IsPresent()
}

func (f *StructField) IsExported() bool {
	return f.IsAnonymous() || ast.IsExported(f.Name.Value)
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

type FuncType struct {
	AST     *ast.FuncType
	Params  []Param
	Results []Param
}

// Param represents a parameter or result field.
type Param struct {
	AST  *ast.Field
	Name Optional[string] // parameter name, or None if a type-only parameter.
	Type Type
}

// TypeParamRefType is a reference to a `TypeParameter` within a declaration block
type TypeParamRefType struct {
	AST *ast.Ident

	Decl  Decl // the declaration this type parameter is defined on
	Index int  // Index into the type parameter slice on the declaration
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
func (FuncType) Family() TypeFamily         { return Func }
func (TypeParamRefType) Family() TypeFamily { return TypeParamRef }

func (t NamedType) ASTExpr() ast.Expr        { return t.AST }
func (t StructType) ASTExpr() ast.Expr       { return t.AST }
func (t MapType) ASTExpr() ast.Expr          { return t.AST }
func (t ListType) ASTExpr() ast.Expr         { return t.AST }
func (t PointerType) ASTExpr() ast.Expr      { return t.AST }
func (t BuiltinType) ASTExpr() ast.Expr      { return t.AST }
func (t FuncType) ASTExpr() ast.Expr         { return t.AST }
func (t TypeParamRefType) ASTExpr() ast.Expr { return t.AST }

type Optional[T any] struct {
	Value   T
	Present bool
}

func (o *Optional[T]) Clear() {
	var zero T
	o.Value = zero
	o.Present = false
}

func (o Optional[T]) String() string {
	if o.Present {
		return fmt.Sprintf("%v", o.Value)
	}
	return "None"
}

func (o Optional[T]) GetOrDefault(def T) T {
	if o.Present {
		return o.Value
	}
	return def
}

func (o Optional[T]) MustGet() (rtn T) {
	if o.Present {
		return o.Value
	}
	panic(errors.Newf("Optional value is not set: %T", rtn))
}

func (o Optional[T]) IsPresent() bool {
	return o.Present
}

func (o Optional[T]) Get() any {
	return o.Value
}

// AsOptional returns an Optional where a zero value T is considered None
// and any other value is considered Some
//
// i.e.
//
//	AsOptional(nil) == None()
//	AsOptional(0) == None()
//	AsOptional(false) == None()
//	AsOptional("") == None()
//	AsOptional(&MyStruct{}) == Some(&MyStruct{})
//	AsOptional(1) == Some(1)
//	AsOptional(true) == Some(true)
func AsOptional[T comparable](v T) Optional[T] {
	var zero T
	if v == zero {
		return None[T]()
	}
	return Some[T](v)
}

// Some returns an Optional with the given value and present set to true
//
// This means Some(nil) is a valid present Optional
// and Some(nil) != None()
func Some[T any](v T) Optional[T] {
	return Optional[T]{Value: v, Present: true}
}

// None returns an Optional with no value set
func None[T any]() Optional[T] {
	return Optional[T]{Present: false}
}
