package schemautil

import (
	"encr.dev/parser2/internal/paths"
	"encr.dev/parser2/internal/schema"
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
func ResolveNamedStruct(t schema.Type, requirePointer bool) (decl *schema.TypeDecl, ok bool) {
	t, derefs := Deref(t)
	if derefs > 1 || (requirePointer && derefs == 0) {
		return nil, false
	}

	if named, ok := t.(schema.NamedType); ok {
		if decl = named.Decl(); decl.Type.Family() == schema.Struct {
			return decl, true
		}
	}
	return nil, false
}
