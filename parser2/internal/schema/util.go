package schema

import (
	"encr.dev/parser2/internal/paths"
)

// IsNamed reports whether a given type is a named type with the given
// package path and name.
func IsNamed(t Type, pkg paths.Pkg, name string) bool {
	if named, ok := t.(NamedType); ok {
		decl := named.DeclInfo
		return decl.Name == name && decl.File.Pkg.ImportPath == pkg
	}
	return false
}

// IsBuiltinKind reports whether the given type is a builtin
// of one of the given kinds.
func IsBuiltinKind(t Type, kinds ...BuiltinKind) bool {
	if b, ok := t.(BuiltinType); ok {
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
func Deref(t Type) (Type, int) {
	n := 0
	for {
		if ptr, ok := t.(PointerType); ok {
			t = ptr.Elem
			n++
			continue
		}
		return t, n
	}
}
