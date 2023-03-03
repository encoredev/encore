package schemautil

import (
	"fmt"
	"reflect"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/schema"
)

// IsNamed reports whether a given type is a named type with the given
// package path and name.
func IsNamed(t schema.Type, pkg paths.Pkg, name string) bool {
	if named, ok := t.(schema.NamedType); ok {
		decl := named.DeclInfo
		return decl.Name == name && decl.File.Pkg.ImportPath == pkg
	}
	return false
}

// IsPointer reports whether t is a pointer type.
func IsPointer(t schema.Type) bool {
	_, ok := t.(schema.PointerType)
	return ok
}

// IsBuiltinKind reports whether the given type is a builtin
// of one of the given kinds.
func IsBuiltinKind(t schema.Type, kinds ...schema.BuiltinKind) bool {
	if b, ok := t.(schema.BuiltinType); ok {
		for _, k := range kinds {
			if b.Kind == k {
				return true
			}
		}
	}
	return false
}

var Signed = []schema.BuiltinKind{
	schema.Int,
	schema.Int8,
	schema.Int16,
	schema.Int32,
	schema.Int64,
}

var Unsigned = []schema.BuiltinKind{
	schema.Uint,
	schema.Uint8,
	schema.Uint16,
	schema.Uint32,
	schema.Uint64,
}

// Integers is a list of all integer builtin kinds.
var Integers = append(append([]schema.BuiltinKind{}, Signed...), Unsigned...)

// Deref dereferences a type until it is not a pointer type.
// It returns the number of pointer dereferences required.
func Deref(t schema.Type) (schema.Type, int) {
	n := 0
	for {
		if ptr, ok := t.(schema.PointerType); ok {
			t = ptr.Elem
			n++
			continue
		}
		return t, n
	}
}

// ResolveNamedStruct reports whether a given type is a named type
// pointing to a struct type.
//
// It always requires at most one pointer dereference, and if
// requirePointer is true it must be exactly one pointer dereference.
//
// If it doesn't match the requirements it returns (nil, false).
func ResolveNamedStruct(t schema.Type, requirePointer bool) (ref *schema.TypeDeclRef, ok bool) {
	t, derefs := Deref(t)
	if derefs > 1 || (requirePointer && derefs == 0) {
		return nil, false
	}

	if named, ok := t.(schema.NamedType); ok {
		if decl := named.Decl(); decl.Type.Family() == schema.Struct {
			return &schema.TypeDeclRef{
				Decl:     decl,
				Pointers: derefs,
				TypeArgs: named.TypeArgs,
			}, true
		}
	}
	return nil, false
}

// ConcretizeGenericType takes a type and applies any type arguments
// into the slots of the type parameters, producing a concrete type.
//
// To be more robust in the presence of typing errors it supports partial application,
// where the number of type arguments may be different than the number of type parameters on the decl.
func ConcretizeGenericType(typ schema.Type) schema.Type {
	return concretize(typ, nil)
}

// ConcretizeWithTypeArgs is like ConcretizeGenericType but operates with
// a list of type arguments. It is used when the type arguments are known
// separately from the type itself, such as when using *schema.TypeDeclRef.
func ConcretizeWithTypeArgs(typ schema.Type, typeArgs []schema.Type) schema.Type {
	return concretize(typ, typeArgs)
}

func concretize(typ schema.Type, typeArgs []schema.Type) schema.Type {
	switch typ := typ.(type) {
	case schema.TypeParamRefType:
		// We have a reference to a type parameter.
		// Is the corresponding type argument in scope? If so replace it.
		if typ.Index < len(typeArgs) {
			return typeArgs[typ.Index]
		} else {
			return typ
		}

	case schema.BuiltinType:
		return typ
	case schema.PointerType:
		return schema.PointerType{AST: typ.AST, Elem: concretize(typ.Elem, typeArgs)}
	case schema.ListType:
		return schema.ListType{AST: typ.AST, Elem: concretize(typ.Elem, typeArgs), Len: typ.Len}
	case schema.MapType:
		return schema.MapType{
			AST:   typ.AST,
			Key:   concretize(typ.Key, typeArgs),
			Value: concretize(typ.Value, typeArgs),
		}
	case schema.StructType:
		result := schema.StructType{
			AST:    nil,
			Fields: make([]schema.StructField, len(typ.Fields)),
		}
		for i, f := range typ.Fields {
			result.Fields[i] = f // copy
			result.Fields[i].Type = concretize(f.Type, typeArgs)
		}
		return result
	case schema.NamedType:
		clone := typ // copy
		for i, arg := range clone.TypeArgs {
			clone.TypeArgs[i] = concretize(arg, typeArgs)
		}
		return clone
	case schema.FuncType:
		clone := typ // copy
		for i, p := range clone.Params {
			clone.Params[i].Type = concretize(p.Type, typeArgs)
		}
		for i, p := range clone.Results {
			clone.Results[i].Type = concretize(p.Type, typeArgs)
		}
		return clone
	case schema.InterfaceType:
		// TODO(andre) we currently don't track any information
		// about interfaces, so nothing to do right now.
		return typ
	default:
		panic(fmt.Sprintf("unknown type %T", typ))
	}
}

type pkgDeclKey struct {
	pkg  paths.Pkg
	name string
}

// Walk performs a depth-first walk of all schema nodes starting at node, calling visitor for each type.
//
// If visitor returns false, the walk is aborted.
func Walk(root schema.Type, visitor func(node schema.Type) bool) {
	declChain := make([]pkgDeclKey, 0, 10)
	walk(root, visitor, declChain)
}

func walk(node schema.Type, visitor func(typ schema.Type) bool, declChain []pkgDeclKey) bool {
	// Check the visitor against the node type
	if !visitor(node) {
		return false
	}

	switch node := node.(type) {
	case schema.NamedType:
		for _, typ := range node.TypeArgs {
			if !walk(typ, visitor, declChain) {
				return false
			}
		}

		// Have we already visited this decl?
		declKey := pkgDeclKey{pkg: node.DeclInfo.File.Pkg.ImportPath, name: node.DeclInfo.Name}
		for i := len(declChain) - 1; i >= 0; i-- {
			if declChain[i] == declKey {
				return true // keep going elsewhere
			}
		}
		declChain = append(declChain, declKey)

		return walk(node.Decl().Type, visitor, declChain)
	case schema.StructType:
		for _, field := range node.Fields {
			if !walk(field.Type, visitor, declChain) {
				return false
			}
		}
	case schema.MapType:
		if !walk(node.Key, visitor, declChain) {
			return false
		}
		return walk(node.Value, visitor, declChain)
	case schema.ListType:
		return walk(node.Elem, visitor, declChain)
	case schema.BuiltinType:
		return true // keep going elsewhere
	case schema.PointerType:
		return walk(node.Elem, visitor, declChain)
	case schema.FuncType:
		for _, part := range [...][]schema.Param{node.Params, node.Results} {
			for _, p := range part {
				if !walk(p.Type, visitor, declChain) {
					return false
				}
			}
		}
		return true
	case schema.InterfaceType:
		return true
	case schema.TypeParamRefType:
		return true
	default:
		panic(fmt.Sprintf("unsupported node type encountered during walk: %+v", reflect.TypeOf(node)))
	}

	return true
}
