package v1

import (
	"fmt"
	"reflect"
)

// Walk will perform a depth first walk of all schema nodes starting at node, calling visitor for each schema type found.
//
// If visitor returns false, the walk will be aborted.
func Walk(decls []*Decl, node any, visitor func(node any) error) error {
	namedChain := make([]uint32, 0, 10)
	return walk(decls, node, visitor, namedChain)
}

func walk(decls []*Decl, node any, visitor func(node any) error, namedChain []uint32) error {
	// Check the visitor against the node type
	if err := visitor(node); err != nil {
		return err
	}

	switch node := node.(type) {
	case *Type:
		switch v := node.Typ.(type) {
		case *Type_Named:
			return walk(decls, v.Named, visitor, namedChain)
		case *Type_Struct:
			return walk(decls, v.Struct, visitor, namedChain)
		case *Type_Map:
			return walk(decls, v.Map, visitor, namedChain)
		case *Type_List:
			return walk(decls, v.List, visitor, namedChain)
		case *Type_Builtin:
			return walk(decls, v.Builtin, visitor, namedChain)
		case *Type_Pointer:
			return walk(decls, v.Pointer, visitor, namedChain)
		case *Type_Option:
			return walk(decls, v.Option, visitor, namedChain)
		case *Type_TypeParameter:
			return walk(decls, v.TypeParameter, visitor, namedChain)
		case *Type_Literal:
			return walk(decls, v.Literal, visitor, namedChain)
		case *Type_Union:
			return walk(decls, v.Union, visitor, namedChain)
		case *Type_Config:
			return walk(decls, v.Config, visitor, namedChain)
		default:
			panic(fmt.Sprintf("unknown type encountered: %+v", reflect.TypeOf(v)))
		}

	case *Decl:
		return walk(decls, decls[node.Id].Type, visitor, namedChain)

	case *Named:
		for _, typ := range node.TypeArguments {
			if err := walk(decls, typ, visitor, namedChain); err != nil {
				return err
			}
		}

		// Have we already visited this named type?
		for i := len(namedChain) - 1; i >= 0; i-- {
			if namedChain[i] == node.Id {
				return nil
			}
		}
		namedChain = append(namedChain, node.Id)

		return walk(decls, decls[node.Id].Type, visitor, namedChain)
	case *Struct:
		for _, field := range node.Fields {
			if err := walk(decls, field.Typ, visitor, namedChain); err != nil {
				return err
			}
		}
	case *Union:
		for _, typ := range node.Types {
			if err := walk(decls, typ, visitor, namedChain); err != nil {
				return err
			}
		}
	case *Map:
		if err := walk(decls, node.Key, visitor, namedChain); err != nil {
			return err
		}
		return walk(decls, node.Value, visitor, namedChain)
	case *List:
		return walk(decls, node.Elem, visitor, namedChain)
	case Builtin:
		return nil
	case *Pointer:
		return walk(decls, node.Base, visitor, namedChain)
	case *Option:
		return walk(decls, node.Value, visitor, namedChain)
	case *TypeParameterRef:
		return nil
	case *Literal:
		return nil
	case *ConfigValue:
		return walk(decls, node.Elem, visitor, namedChain)
	default:
		panic(fmt.Sprintf("unsupported node type encountered during walk: %+v", reflect.TypeOf(node)))
	}

	return nil
}
