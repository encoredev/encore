package v1

import (
	"fmt"
	"reflect"
)

// Walk will perform a depth first walk of all schema nodes starting at node, calling visitor for each schema type found.
//
// If visitor returns false, the walk will be aborted.
func Walk(decls []*Decl, node any, visitor func(node any) error) error {
	// Check the visitor against the node type
	if err := visitor(node); err != nil {
		return err
	}

	switch node := node.(type) {
	case *Type:
		switch v := node.Typ.(type) {
		case *Type_Named:
			return Walk(decls, v.Named, visitor)
		case *Type_Struct:
			return Walk(decls, v.Struct, visitor)
		case *Type_Map:
			return Walk(decls, v.Map, visitor)
		case *Type_List:
			return Walk(decls, v.List, visitor)
		case *Type_Builtin:
			return Walk(decls, v.Builtin, visitor)
		case *Type_Pointer:
			return Walk(decls, v.Pointer, visitor)
		case *Type_TypeParameter:
			return Walk(decls, v.TypeParameter, visitor)
		case *Type_Config:
			return Walk(decls, v.Config, visitor)
		default:
			panic(fmt.Sprintf("unknown type encountered: %+v", reflect.TypeOf(v)))
		}

	case *Decl:
		return Walk(decls, decls[node.Id].Type, visitor)

	case *Named:
		for _, typ := range node.TypeArguments {
			if err := Walk(decls, typ, visitor); err != nil {
				return err
			}
		}
		return Walk(decls, decls[node.Id].Type, visitor)
	case *Struct:
		for _, field := range node.Fields {
			if err := Walk(decls, field.Typ, visitor); err != nil {
				return err
			}
		}
	case *Map:
		if err := Walk(decls, node.Key, visitor); err != nil {
			return err
		}
		return Walk(decls, node.Value, visitor)
	case *List:
		return Walk(decls, node.Elem, visitor)
	case Builtin:
		return nil
	case *Pointer:
		return Walk(decls, node.Base, visitor)
	case *TypeParameterRef:
		return nil
	case *ConfigValue:
		return Walk(decls, node.Elem, visitor)
	default:
		panic(fmt.Sprintf("unsupported node type encountered during walk: %+v", reflect.TypeOf(node)))
	}

	return nil
}
