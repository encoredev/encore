package schemautil

import (
	"go/ast"
	"slices"
	"strings"

	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/schema"
)

// FlatField is a struct field as encoding/json sees it,
// with the fields of embedded structs promoted.
type FlatField struct {
	Field schema.StructField   // the field; its Name is always set
	Path  []schema.StructField // embedded fields leading to Field, outermost first
}

// IsPromoted reports whether the field was promoted from an embedded struct.
func (f FlatField) IsPromoted() bool {
	return len(f.Path) > 0
}

// ThroughPointer reports whether the field is reached through an embedded pointer.
func (f FlatField) ThroughPointer() bool {
	for _, p := range f.Path {
		if _, ok := p.Type.(schema.PointerType); ok {
			return true
		}
	}
	return false
}

// GoPath returns the embedded field names to select through to reach Field.
// Unexported names are omitted, since Go promotes fields through them.
func (f FlatField) GoPath() []string {
	var names []string
	for _, p := range f.Path {
		if name, ok := EmbeddedFieldName(p); ok && ast.IsExported(name) {
			names = append(names, name)
		}
	}
	return names
}

// EmbeddedFieldName returns the Go name of an embedded field, which is its type name.
func EmbeddedFieldName(f schema.StructField) (string, bool) {
	if !f.IsAnonymous() {
		return "", false
	}
	t, _ := Deref(f.Type)
	if named, ok := t.(schema.NamedType); ok {
		return named.DeclInfo.Name, true
	}
	return "", false
}

// FlattenStruct returns the fields of st as encoding/json encodes them,
// with the fields of embedded structs promoted, in declaration order.
// A struct without embedded fields is returned unchanged.
func FlattenStruct(errs *perr.List, st schema.StructType) []FlatField {
	return ResolveConflicts(PromotedFields(errs, st), func(f FlatField) (FlatField, option.Option[ConflictName]) {
		return f, JSONName(f)
	})
}

// PromotedFields returns the fields of st with the fields of embedded structs promoted,
// in declaration order, without resolving name conflicts; see ResolveConflicts.
func PromotedFields(errs *perr.List, st schema.StructType) []FlatField {
	if !slices.ContainsFunc(st.Fields, func(f schema.StructField) bool { return f.IsAnonymous() }) {
		result := make([]FlatField, len(st.Fields))
		for i, f := range st.Fields {
			result[i] = FlatField{Field: f}
		}
		return result
	}

	var out []FlatField
	collectFlatFields(errs, st, nil, nil, &out)
	return out
}

func collectFlatFields(errs *perr.List, st schema.StructType, path []schema.StructField, seen []pkgDeclKey, out *[]FlatField) {
	for _, f := range st.Fields {
		_, tagged, omit := jsonFieldTag(f)

		name, named := f.Name.Get()
		if !named {
			if omit {
				continue
			}

			typeName, ok := EmbeddedFieldName(f)
			if !ok {
				continue
			}

			elem, _ := Deref(f.Type)
			named := elem.(schema.NamedType)
			if !tagged {
				if st, ok := named.Decl().Type.(schema.StructType); ok {
					// Stop at embedding cycles, such as struct{ *T }.
					key := pkgDeclKey{pkg: named.DeclInfo.File.Pkg.ImportPath, name: named.DeclInfo.Name}
					if slices.Contains(seen, key) {
						continue
					}

					concrete := ConcretizeWithTypeArgs(errs, st, named.TypeArgs).(schema.StructType)
					collectFlatFields(errs, concrete, append(slices.Clone(path), f), append(slices.Clone(seen), key), out)
					continue
				}
			}

			// Encoded as a single field named after its type.
			if !tagged && !ast.IsExported(typeName) {
				continue
			}
			f.Name = option.Some(typeName)
		} else if len(path) > 0 && !ast.IsExported(name) {
			continue
		}

		*out = append(*out, FlatField{Field: f, Path: path})
	}
}

// ConflictName is the name a field is known by when resolving name conflicts.
type ConflictName struct {
	Name   string
	Tagged bool // set by a tag, which breaks ties between fields at the same depth
}

// JSONName returns the name encoding/json encodes f under.
// Fields tagged `json:"-"` have none, so they never conflict.
func JSONName(f FlatField) option.Option[ConflictName] {
	tagName, tagged, omit := jsonFieldTag(f.Field)
	switch {
	case omit:
		return option.None[ConflictName]()
	case tagged:
		return option.Some(ConflictName{Name: tagName, Tagged: true})
	default:
		return option.Some(ConflictName{Name: f.Field.Name.MustGet()})
	}
}

// ResolveConflicts drops the items whose field loses a name conflict, as encoding/json does:
// the shallowest field wins, then the only tagged one; otherwise all of them are dropped.
// Only conflicts involving a promoted field are resolved, to keep existing behavior.
// key returns an item's field and the name it is known by, if any.
func ResolveConflicts[T any](items []T, key func(T) (FlatField, option.Option[ConflictName])) []T {
	type entry struct {
		depth    int
		tagged   bool
		promoted bool
	}
	entries := make([]entry, len(items))
	byName := make(map[string][]int, len(items))
	for i, item := range items {
		f, name := key(item)
		entries[i] = entry{depth: len(f.Path), promoted: f.IsPromoted()}
		if n, ok := name.Get(); ok {
			entries[i].tagged = n.Tagged
			byName[n.Name] = append(byName[n.Name], i)
		}
	}

	drop := make([]bool, len(items))
	for _, group := range byName {
		if len(group) < 2 || !slices.ContainsFunc(group, func(i int) bool { return entries[i].promoted }) {
			continue
		}

		minDepth := entries[group[0]].depth
		for _, i := range group {
			minDepth = min(minDepth, entries[i].depth)
		}
		var shallowest, tagged []int
		for _, i := range group {
			if entries[i].depth == minDepth {
				shallowest = append(shallowest, i)
				if entries[i].tagged {
					tagged = append(tagged, i)
				}
			}
		}

		winner := -1
		if len(shallowest) == 1 {
			winner = shallowest[0]
		} else if len(tagged) == 1 {
			winner = tagged[0]
		}
		for _, i := range group {
			drop[i] = i != winner
		}
	}

	var kept []T
	for i, item := range items {
		if !drop[i] {
			kept = append(kept, item)
		}
	}
	return kept
}

// jsonFieldTag returns the field's json tag name, and whether it is `json:"-"`.
func jsonFieldTag(f schema.StructField) (name string, tagged, omit bool) {
	tag, err := f.Tag.Get("json")
	if err != nil {
		return "", false, false
	}
	if tag.Name == "-" && len(tag.Options) == 0 {
		return "", false, true
	}
	name = strings.TrimSpace(tag.Name)
	return name, name != "", false
}
