package cuegen

import (
	"fmt"
	"reflect"
	"strings"

	"cuelang.org/go/cue/ast"

	schema "encr.dev/proto/encore/parser/schema/v1"
)

// definitionGenerator is used to count the number of types a specific named type
// is used (named types include type arguements, such that Option[string] != Option[int]).
//
// It also counts the number of times a unique named type uses a decl.
//
// This allows us to:
// - determine if we should inline a named type into a config field if it's only used once
// - generate a unique name for a generic decl if it's used with multiple type arguments
type definitionGenerator struct {
	decls          []*schema.Decl
	ids            []*schema.Named
	definitionName map[int]string // id -> name
	nameCount      map[string]int // name -> usage count
	counts         map[int]int
}

func newDefinitionGenerator(decls []*schema.Decl) *definitionGenerator {
	return &definitionGenerator{
		decls:          decls,
		ids:            nil,
		definitionName: make(map[int]string),
		nameCount:      make(map[string]int),
		counts:         make(map[int]int),
	}
}

// ID returns the id for the named type. If two named takes are passed in
// which are identical, it will return the same id.
//
// This function then also creates a unique definition name for the named type
// which we could use within the generated CUE file
func (n *definitionGenerator) ID(named *schema.Named) int {
	for idx, other := range n.ids {
		if reflect.DeepEqual(named, other) {
			return idx
		}
	}

	// Assign a new ID
	n.ids = append(n.ids, named)
	id := len(n.ids) - 1

	// Create a unique name for this definition
	defaultName := n.typeToDefinitionName(&schema.Type{Typ: &schema.Type_Named{Named: named}})
	usageCount, found := n.nameCount[defaultName]
	if !found {
		n.definitionName[id] = defaultName
	} else {
		n.definitionName[id] = fmt.Sprintf("%s_%d", defaultName, usageCount)
	}
	n.nameCount[defaultName] = usageCount + 1

	return id
}

func (n *definitionGenerator) CueIdent(named *schema.Named) *ast.Ident {
	return ast.NewIdent("#" + n.definitionName[n.ID(named)])
}

func (n *definitionGenerator) Inc(named *schema.Named) {
	id := n.ID(named)
	n.counts[id]++
}

func (n *definitionGenerator) Count(named *schema.Named) int {
	id := n.ID(named)
	return n.counts[id]
}

func (n *definitionGenerator) NamesWithCountsOver(x int) []*schema.Named {
	rtn := make([]*schema.Named, 0, len(n.ids))
	for id, name := range n.ids {
		if n.counts[id] > x {
			rtn = append(rtn, name)
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
func (n *definitionGenerator) typeToDefinitionName(typ *schema.Type) string {
	switch typ := typ.Typ.(type) {
	case *schema.Type_Named:
		var name strings.Builder
		name.WriteString(n.decls[typ.Named.Id].Name)
		for _, typeArg := range typ.Named.TypeArguments {
			name.WriteString("_")
			name.WriteString(n.typeToDefinitionName(typeArg))
		}
		return name.String()
	case *schema.Type_List:
		return "List_" + n.typeToDefinitionName(typ.List.Elem)
	case *schema.Type_Map:
		return "Map_" + n.typeToDefinitionName(typ.Map.Key) + "_" + n.typeToDefinitionName(typ.Map.Value)
	case *schema.Type_Config:
		return n.typeToDefinitionName(typ.Config.Elem)
	case *schema.Type_Builtin:
		switch typ.Builtin {
		case schema.Builtin_ANY:
			return "any"
		case schema.Builtin_BOOL:
			return "bool"
		case schema.Builtin_INT8:
			return "int8"
		case schema.Builtin_INT16:
			return "int16"
		case schema.Builtin_INT32:
			return "int32"
		case schema.Builtin_INT64:
			return "int64"
		case schema.Builtin_UINT8:
			return "uint8"
		case schema.Builtin_UINT16:
			return "uint16"
		case schema.Builtin_UINT32:
			return "uint32"
		case schema.Builtin_UINT64:
			return "uint64"
		case schema.Builtin_FLOAT32:
			return "float32"
		case schema.Builtin_FLOAT64:
			return "float64"
		case schema.Builtin_STRING:
			return "string"
		case schema.Builtin_BYTES:
			return "bytes"
		case schema.Builtin_TIME:
			return "string"
		case schema.Builtin_UUID:
			return "string"
		case schema.Builtin_JSON:
			return "string"
		case schema.Builtin_USER_ID:
			return "string"
		case schema.Builtin_INT:
			return "int"
		case schema.Builtin_UINT:
			return "uint"
		default:
			return ""
		}
	}

	return ""
}
