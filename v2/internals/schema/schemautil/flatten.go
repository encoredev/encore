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
	if !slices.ContainsFunc(st.Fields, func(f schema.StructField) bool { return f.IsAnonymous() }) {
		result := make([]FlatField, len(st.Fields))
		for i, f := range st.Fields {
			result[i] = FlatField{Field: f}
		}
		return result
	}

	var candidates []flatCandidate
	collectFlatFields(errs, st, nil, nil, nil, &candidates)
	return resolveFlatConflicts(candidates)
}

// flatCandidate is a field found while flattening, before conflicts are resolved.
type flatCandidate struct {
	FlatField
	index    []int // field index at each level, for ordering
	jsonName string
	tagged   bool // jsonName came from a json tag
}

func collectFlatFields(errs *perr.List, st schema.StructType, path []schema.StructField, index []int, seen []pkgDeclKey, out *[]flatCandidate) {
	for i, f := range st.Fields {
		idx := append(slices.Clone(index), i)
		tagName, tagged, omit := jsonFieldTag(f)

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
					collectFlatFields(errs, concrete, append(slices.Clone(path), f), idx, append(slices.Clone(seen), key), out)
					continue
				}
			}

			// Encoded as a single field named after its type.
			if !tagged && !ast.IsExported(typeName) {
				continue
			}
			f.Name = option.Some(typeName)
			name = typeName
		} else if len(path) > 0 && !ast.IsExported(name) {
			continue
		}

		jsonName := name
		if tagged {
			jsonName = tagName
		}
		*out = append(*out, flatCandidate{
			FlatField: FlatField{Field: f, Path: path},
			index:     idx,
			jsonName:  jsonName,
			tagged:    tagged,
		})
	}
}

// resolveFlatConflicts drops fields that lose a JSON name conflict, as encoding/json does,
// and returns the rest in declaration order.
func resolveFlatConflicts(candidates []flatCandidate) []FlatField {
	byName := make(map[string][]int, len(candidates))
	for i, c := range candidates {
		byName[c.jsonName] = append(byName[c.jsonName], i)
	}

	drop := make([]bool, len(candidates))
	for _, group := range byName {
		if len(group) < 2 {
			continue
		}

		// Only resolve conflicts involving promoted fields, to keep existing behavior.
		if !slices.ContainsFunc(group, func(i int) bool { return candidates[i].IsPromoted() }) {
			continue
		}

		minDepth := len(candidates[group[0]].Path)
		for _, i := range group {
			minDepth = min(minDepth, len(candidates[i].Path))
		}
		var shallowest, tagged []int
		for _, i := range group {
			if len(candidates[i].Path) == minDepth {
				shallowest = append(shallowest, i)
				if candidates[i].tagged {
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

	var kept []flatCandidate
	for i, c := range candidates {
		if !drop[i] {
			kept = append(kept, c)
		}
	}
	slices.SortStableFunc(kept, func(a, b flatCandidate) int {
		return slices.Compare(a.index, b.index)
	})

	result := make([]FlatField, len(kept))
	for i, c := range kept {
		result[i] = c.FlatField
	}
	return result
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
