package schemautil

import (
	"fmt"

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
