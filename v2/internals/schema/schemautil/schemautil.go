package schemautil

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"slices"
	"strconv"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
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

// IsOption reports whether t is an option type.
func IsOption(t schema.Type) bool {
	_, ok := t.(schema.OptionType)
	return ok
}

// IsBuiltinKind reports whether the given type is a builtin
// of one of the given kinds.
func IsBuiltinKind(t schema.Type, kinds ...schema.BuiltinKind) bool {
	if b, ok := t.(schema.BuiltinType); ok {
		if slices.Contains(kinds, b.Kind) {
			return true
		}
	}
	return false
}

// IsBuiltinOrList reports whether the given type is a builtin,
// a list of builtins, or neither.
func IsBuiltinOrList(t schema.Type) (kind schema.BuiltinKind, isList bool, ok bool) {
	if b, ok := t.(schema.BuiltinType); ok {
		return b.Kind, false, true
	}
	if list, ok := t.(schema.ListType); ok {
		if b, ok := list.Elem.(schema.BuiltinType); ok {
			return b.Kind, true, true
		}
	}
	return schema.Invalid, false, false
}

// IsValidHeaderType reports whether the given type is valid for use as an HTTP header value.
func IsValidHeaderType(t schema.Type) bool {
	if list, ok := t.(schema.ListType); ok {
		return isValidHeaderTypeValue(list.Elem)
	}
	return isValidHeaderTypeValue(t)
}

func isValidHeaderTypeValue(t schema.Type) bool {
	switch t := t.(type) {
	case schema.BuiltinType:
		return true
	case schema.OptionType:
		return isValidHeaderTypeValue(t.Value)
	default:
		return false
	}
}

// IsValidQueryType reports whether the given type is valid for use as a query parameter value.
func IsValidQueryType(t schema.Type) bool {
	if list, ok := t.(schema.ListType); ok {
		return isValidQueryTypeValue(list.Elem)
	}
	return isValidQueryTypeValue(t)
}

func isValidQueryTypeValue(t schema.Type) bool {
	switch t := t.(type) {
	case schema.BuiltinType:
		return true
	case schema.OptionType:
		return isValidHeaderTypeValue(t.Value)
	default:
		return false
	}
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

// DerefNamedInfo returns what package declaration a given named type references, if any.
//
// It always requires at most one pointer dereference, and if
// requirePointer is true it must be exactly one pointer dereference.
//
// If the type is not a named type or otherwise doesn't match the requirements, it returns (nil, false).
func DerefNamedInfo(t schema.Type, requirePointer bool) (info *pkginfo.PkgDeclInfo, ok bool) {
	t, derefs := Deref(t)
	if derefs > 1 || (requirePointer && derefs == 0) {
		return nil, false
	}
	if named, ok := t.(schema.NamedType); ok {
		return named.DeclInfo, true
	}
	return nil, false
}

// ConcretizeGenericType takes a type and applies any type arguments
// into the slots of the type parameters, producing a concrete type.
//
// To be more robust in the presence of typing errors it supports partial application,
// where the number of type arguments may be different than the number of type parameters on the decl.
func ConcretizeGenericType(errs *perr.List, typ schema.Type) schema.Type {
	return concretize(errs, typ.ASTExpr(), typ, nil, nil)
}

// ConcretizeWithTypeArgs is like ConcretizeGenericType but operates with
// a list of type arguments. It is used when the type arguments are known
// separately from the type itself, such as when using *schema.TypeDeclRef.
func ConcretizeWithTypeArgs(errs *perr.List, typ schema.Type, typeArgs []schema.Type) schema.Type {
	return concretize(errs, typ.ASTExpr(), typ, typeArgs, nil)
}

func concretize(errs *perr.List, referencedFrom ast.Node, typ schema.Type, typeArgs []schema.Type, seenDecls map[TypeHash]*schema.TypeDecl) schema.Type {
	// seenDecls is used to avoid infinite recursion
	// for mutually recursive types.
	if seenDecls == nil {
		seenDecls = make(map[TypeHash]*schema.TypeDecl)
	}

	switch typ := typ.(type) {
	case schema.TypeParamRefType:
		// We have a reference to a type parameter.
		// Is the corresponding type argument in scope? If so replace it.
		if typ.Index < len(typeArgs) {
			return typeArgs[typ.Index]
		} else {
			errs.Add(
				errMissingTypeArg.
					AtGoNode(typ.AST),
			)
			return typ // return the type param ref as a placeholder
		}

	case schema.BuiltinType:
		return typ
	case schema.PointerType:
		return schema.PointerType{AST: typ.AST, Elem: concretize(errs, typ.AST, typ.Elem, typeArgs, seenDecls)}
	case schema.OptionType:
		return schema.OptionType{AST: typ.AST, Value: concretize(errs, typ.AST, typ.Value, typeArgs, seenDecls)}
	case schema.ListType:
		return schema.ListType{AST: typ.AST, Elem: concretize(errs, typ.AST.Elt, typ.Elem, typeArgs, seenDecls), Len: typ.Len}
	case schema.MapType:
		return schema.MapType{
			AST:   typ.AST,
			Key:   concretize(errs, typ.AST.Key, typ.Key, typeArgs, seenDecls),
			Value: concretize(errs, typ.AST.Value, typ.Value, typeArgs, seenDecls),
		}
	case schema.StructType:
		result := schema.StructType{
			AST:    typ.AST,
			Fields: make([]schema.StructField, len(typ.Fields)),
		}
		for i, f := range typ.Fields {
			result.Fields[i] = f // copy
			result.Fields[i].Type = concretize(errs, f.AST.Type, f.Type, typeArgs, seenDecls)
		}
		return result
	case schema.NamedType:
		if typParams := typ.Decl().TypeParams; len(typParams) > len(typ.TypeArgs) {
			numMissing := len(typParams) - len(typ.TypeArgs)
			idx := len(typParams) - numMissing
			errs.Add(
				errMissingTypeArg.
					AtGoNode(referencedFrom, errors.AsError("missing from here")).
					AtGoNode(typParams[idx].AST, errors.AsHelp("missing type parameter defined here")),
			)
			return typ // return the named type as a placeholder
		}

		// Clone the named type. Clone the slice so we don't overwrite the original.
		clone := typ
		clone.TypeArgs = slices.Clone(typ.TypeArgs)

		for i, arg := range clone.TypeArgs {
			clone.TypeArgs[i] = concretize(errs, typ.AST, arg, typeArgs, seenDecls)
		}

		// If we've already seen this declaration, don't clone it to avoid
		// infinite recursion.
		hash := Hash(typ)
		if decl, ok := seenDecls[hash]; ok {
			return clone.WithDecl(decl)
		}

		decl := clone.Decl().Clone()

		// Clone and concretize the declaration.
		seenDecls[hash] = decl
		decl.Type = concretize(errs, typ.AST, decl.Type, clone.TypeArgs, seenDecls)
		return clone.WithDecl(decl)

	case schema.FuncType:
		// Clone the function type. Clone the slices so we don't overwrite the original.
		clone := typ // copy
		clone.Params = slices.Clone(typ.Params)
		clone.Results = slices.Clone(typ.Results)

		for i, p := range clone.Params {
			clone.Params[i].Type = concretize(errs, p.AST.Type, p.Type, typeArgs, seenDecls)
		}
		for i, p := range clone.Results {
			clone.Results[i].Type = concretize(errs, p.AST.Type, p.Type, typeArgs, seenDecls)
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
	case schema.OptionType:
		return walk(node.Value, visitor, declChain)
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

type TypeHash [32]byte

// Hash produces a hash of the given type.
// Identical types return identical hashes.
func Hash(typ schema.Type) TypeHash {
	var buf bytes.Buffer
	hashType(&buf, typ)
	return sha256.Sum256(buf.Bytes())
}

func hashType(buf *bytes.Buffer, t schema.Type) {
	switch t := t.(type) {
	case schema.NamedType:
		buf.WriteString("named:")
		buf.WriteString(t.DeclInfo.File.Pkg.ImportPath.String())
		buf.WriteString(".")
		buf.WriteString(t.DeclInfo.Name)
		if len(t.TypeArgs) > 0 {
			buf.WriteString("[")
			for i, arg := range t.TypeArgs {
				if i > 0 {
					buf.WriteString(", ")
				}
				hashType(buf, arg)
			}
			buf.WriteString("]")
		}

	case schema.StructType:
		buf.WriteString("struct{")
		for _, f := range t.Fields {
			if name, ok := f.Name.Get(); ok {
				buf.WriteString(name)
				hashType(buf, f.Type)
				buf.WriteString(";")
			}
		}
		buf.WriteString("}")

	case schema.MapType:
		buf.WriteString("map[")
		hashType(buf, t.Key)
		buf.WriteString("]")
		hashType(buf, t.Value)

	case schema.ListType:
		if t.Len >= 0 {
			fmt.Fprintf(buf, "[%d]", t.Len)
		} else {
			buf.WriteString("[]")
		}
		hashType(buf, t.Elem)

	case schema.PointerType:
		buf.WriteString("*")
		hashType(buf, t.Elem)

	case schema.OptionType:
		buf.WriteString("Option[")
		hashType(buf, t.Value)
		buf.WriteString("]")

	case schema.FuncType:
		buf.WriteString("func(")
		for i, p := range t.Params {
			if i > 0 {
				buf.WriteString(", ")
			}
			hashType(buf, p.Type)
		}
		buf.WriteString(")")

		if len(t.Results) > 0 {
			buf.WriteString(" (")
			for i, p := range t.Results {
				if i > 0 {
					buf.WriteString(", ")
				}
				hashType(buf, p.Type)
			}
			buf.WriteString(")")
		}

	case schema.InterfaceType:
		// We don't track interface methods yet, so outsource
		// this to go/types for now.
		types.WriteExpr(buf, t.AST)

	case schema.BuiltinType:
		buf.WriteString(t.String())

	case schema.TypeParamRefType:
		buf.WriteString("typeparamref:")
		buf.WriteString(t.Decl.DeclaredIn().Pkg.ImportPath.String())
		buf.WriteString(".")
		if name, ok := t.Decl.PkgName().Get(); ok {
			buf.WriteString(name)
		} else {
			buf.WriteString("anon")
		}
		buf.WriteString("#")
		buf.WriteString(strconv.Itoa(t.Index))

	default:
		panic(fmt.Sprintf("unknown type %T", t))
	}
}

// UnwrapConfigType unwraps a config.Value[T] or config.Values[T] type to T or []T respectively.
// If the type is not a config.Value[T] or config.Values[T] type, it returns the type unchanged.
// If there are any errors encountered they are reported to errs.
func UnwrapConfigType(errs *perr.List, t schema.NamedType) (typ schema.Type, isList, isConfig bool) {
	if t.DeclInfo.File.Pkg.ImportPath != "encore.dev/config" {
		return t, false, false
	}

	if t.DeclInfo.Name == "Values" {
		if len(t.TypeArgs) == 0 {
			// Invalid use of config.Values[T]
			errs.AddPos(t.AST.Pos(), "invalid use of config.Values[T]: no type arguments provided")
			return schema.BuiltinType{Kind: schema.Invalid}, true, true
		}

		return t.TypeArgs[0], true, true
	} else if t.DeclInfo.Name == "Value" {
		if len(t.TypeArgs) == 0 {
			// Invalid use of config.Value[T]
			errs.AddPos(t.AST.Pos(), "invalid use of config.Value[T]: no type arguments provided")
			return schema.BuiltinType{Kind: schema.Invalid}, false, true
		}
		return t.TypeArgs[0], false, true
	} else {
		// Use of some helper type alias, like config.Bool
		decl := t.Decl()
		if named, ok := decl.Type.(schema.NamedType); ok && named.DeclInfo.Name == "Value" {
			if len(named.TypeArgs) == 0 {
				// Invalid use of config.Value[T]
				errs.AddPos(t.AST.Pos(), "invalid use of config.Value[T]: no type arguments provided")
				return schema.BuiltinType{Kind: schema.Invalid}, false, true
			}
			return named.TypeArgs[0], false, true
		} else {
			// Invalid use of config.Value[T]
			errs.Addf(t.AST.Pos(), "unrecognized config type: %s", t.DeclInfo.Name)
			return schema.BuiltinType{Kind: schema.Invalid}, false, true
		}
	}
}
