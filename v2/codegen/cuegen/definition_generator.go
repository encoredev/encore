package cuegen

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue/ast"

	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
)

// definitionGenerator is used to count the number of types a specific named type
// is used (named types include type arguments, such that Option[string] != Option[int]).
//
// It also counts the number of times a unique named type uses a decl.
//
// This allows us to:
// - determine if we should inline a named type into a config field if it's only used once
// - generate a unique name for a generic decl if it's used with multiple type arguments
type definitionGenerator struct {
	seenTypes      map[schemautil.TypeHash]schema.NamedType
	definitionName map[schemautil.TypeHash]string // type -> name
	counts         map[schemautil.TypeHash]int    // type -> usage count
	nameCount      map[string]int                 // name -> usage count for base name
}

func newDefinitionGenerator() *definitionGenerator {
	return &definitionGenerator{
		seenTypes:      make(map[schemautil.TypeHash]schema.NamedType),
		definitionName: make(map[schemautil.TypeHash]string),
		nameCount:      make(map[string]int),
		counts:         make(map[schemautil.TypeHash]int),
	}
}

// ID returns the id for the named type. If two named types are passed in
// which are identical, it will return the same id.
//
// This function then also creates a unique definition name for the named type
// which we could use within the generated CUE file
func (n *definitionGenerator) hash(named schema.NamedType) schemautil.TypeHash {
	hash := schemautil.Hash(named)
	if _, ok := n.seenTypes[hash]; ok {
		return hash
	}

	// Create a unique name for this definition
	defaultName := n.typeToDefinitionName(named)
	usageCount, found := n.nameCount[defaultName]
	if !found {
		n.definitionName[hash] = defaultName
	} else {
		n.definitionName[hash] = fmt.Sprintf("%s_%d", defaultName, usageCount)
	}
	n.nameCount[defaultName] = usageCount + 1

	return hash
}

func (n *definitionGenerator) CueIdent(named schema.NamedType) *ast.Ident {
	return ast.NewIdent("#" + n.definitionName[n.hash(named)])
}

func (n *definitionGenerator) Inc(named schema.NamedType) {
	id := n.hash(named)
	n.counts[id]++
}

func (n *definitionGenerator) Count(named schema.NamedType) int {
	id := n.hash(named)
	return n.counts[id]
}

func (n *definitionGenerator) NamesWithCountsOver(x int) []schema.NamedType {
	rtn := make([]schema.NamedType, 0, len(n.seenTypes))
	for hash, typ := range n.seenTypes {
		if n.counts[hash] > x {
			rtn = append(rtn, typ)
		}
	}
	return rtn
}

// typeToDefinitionName converts a schema type into a possible name we could
// use for the CUE definition.
//
// It will not return anything for inline structs.
// It will include type arguments in the name so the same decl created with
// different types will result in different names.
func (n *definitionGenerator) typeToDefinitionName(typ schema.Type) string {
	switch typ := typ.(type) {
	case schema.NamedType:
		var name strings.Builder
		name.WriteString(typ.DeclInfo.Name)
		for _, typeArg := range typ.TypeArgs {
			name.WriteString("_")
			name.WriteString(n.typeToDefinitionName(typeArg))
		}
		return name.String()
	case schema.ListType:
		return "List_" + n.typeToDefinitionName(typ.Elem)
	case schema.MapType:
		return "Map_" + n.typeToDefinitionName(typ.Key) + "_" + n.typeToDefinitionName(typ.Value)
	case schema.PointerType:
		return n.typeToDefinitionName(typ.Elem)
	case schema.BuiltinType:
		switch typ.Kind {
		case schema.Any:
			return "any"
		case schema.Bool:
			return "bool"
		case schema.Int8:
			return "int8"
		case schema.Int16:
			return "int16"
		case schema.Int32:
			return "int32"
		case schema.Int64:
			return "int64"
		case schema.Uint8:
			return "uint8"
		case schema.Uint16:
			return "uint16"
		case schema.Uint32:
			return "uint32"
		case schema.Uint64:
			return "uint64"
		case schema.Float32:
			return "float32"
		case schema.Float64:
			return "float64"
		case schema.String:
			return "string"
		case schema.Bytes:
			return "bytes"
		case schema.Time:
			return "string"
		case schema.UUID:
			return "string"
		case schema.JSON:
			return "string"
		case schema.UserID:
			return "string"
		case schema.Int:
			return "int"
		case schema.Uint:
			return "uint"
		default:
			return ""
		}
	}

	return ""
}
