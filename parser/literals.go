package parser

import (
	"errors"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/runtimeconstants"
)

// litString will return the string value of a given node
//
// If the given node isn't a string literal, it will return an empty string and false
func litString(node ast.Node) (string, bool) {
	if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return lit.Value[1 : len(lit.Value)-1], true
	}
	return "", false
}

func (p *parser) parseStructLit(file *est.File, expectedType string, node ast.Expr) (constants map[string]any, dynamic map[string]ast.Expr) {
	cl, ok := node.(*ast.CompositeLit)
	if !ok {
		p.errf(node.Pos(), "Expected a literal instance of `%s`, got %s.", expectedType, prettyPrint(node))
		return nil, nil
	}

	constants = make(map[string]any)
	dynamic = make(map[string]ast.Expr)

elemLoop:
	for _, elem := range cl.Elts {
		switch elem := elem.(type) {
		case *ast.KeyValueExpr:
			ident, ok := elem.Key.(*ast.Ident)
			if !ok {
				p.errf(elem.Key.Pos(), "Expected a key to be an identifier, got a %v", reflect.TypeOf(elem.Key))
				continue elemLoop
			}
			if ident == nil {
				p.err(elem.Key.Pos(), "Expected a key to be an identifier, got a nil")
				continue elemLoop
			}

			switch value := elem.Value.(type) {
			case *ast.BasicLit:
				v, err := basicLit(value)
				if err != nil {
					p.errf(value.Pos(), "Unable to parse the basic literal: %v", err)
				} else {
					constants[ident.Name] = v
				}

			case *ast.SelectorExpr:
				pkg, obj := pkgObj(p.names[file.Pkg].Files[file], value)
				if pkg != "" {
					if value, found := runtimeconstants.Get(pkg, obj); found {
						constants[ident.Name] = value
						continue elemLoop
					}
				}

				dynamic[ident.Name] = value
			default:
				dynamic[ident.Name] = value
			}
		default:
			p.errf(elem.Pos(), "Expected a key-value pair, got a %v", reflect.TypeOf(elem))
		}
	}

	return
}

func basicLit(value *ast.BasicLit) (any, error) {
	switch value.Kind {
	case token.IDENT:
		return value.Value, nil
	case token.INT:
		return strconv.Atoi(value.Value)
	case token.FLOAT:
		return strconv.ParseFloat(value.Value, 64)
	case token.IMAG:
		return strconv.ParseComplex(value.Value, 64)
	case token.CHAR:
		c, _, _, err := strconv.UnquoteChar(value.Value, value.Value[0])
		return c, err
	case token.STRING:
		return value.Value[1 : len(value.Value)-1], nil
	default:
		return nil, errors.New("unsupported literal type")
	}
}
