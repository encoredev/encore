package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
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

			// Parse any sub data structures
			switch value := elem.Value.(type) {
			case *ast.UnaryExpr:
				if value.Op == token.AND {
					if compositeLiteral, ok := value.X.(*ast.CompositeLit); ok {
						subConstants, subDynamic := p.parseStructLit(file, "struct", compositeLiteral)

						if len(subDynamic) > 0 {
							p.errf(compositeLiteral.Pos(), "Dynamic structs are not supported in constant values")
						} else {
							constants[ident.Name] = subConstants
						}

						continue elemLoop
					}
				}

			case *ast.CompositeLit:
				subConstants, subDynamic := p.parseStructLit(file, "struct", value)

				if len(subDynamic) > 0 {
					p.errf(value.Pos(), "Dynamic structs are not supported in constant values")
				} else {
					constants[ident.Name] = subConstants
				}

				continue elemLoop
			}

			// Parse the value
			value := p.parseConstantValue(file, elem.Value)
			if value.Kind() == constant.Unknown {
				if ident.Name != "Handler" {
					p.errf(elem.Value.Pos(), "Expected a literal value, got %s for %s.", prettyPrint(elem.Value), ident.Name)
				}
				dynamic[ident.Name] = elem.Value
			} else {
				constants[ident.Name] = constant.Val(value)
			}
		default:
			p.errf(elem.Pos(), "Expected a key-value pair, got a %v", reflect.TypeOf(elem))
		}
	}

	return
}

func (p *parser) parseConstantValue(file *est.File, value ast.Expr) (rtn constant.Value) {
	defer func() {
		if r := recover(); r != nil {
			p.errf(value.Pos(), "Panicked while parsing constant value: %v", r)
			rtn = constant.MakeUnknown()
		}
	}()

	switch value := value.(type) {
	case *ast.FuncLit:
		// Functions are not literal constant values
		return constant.MakeUnknown()

	case *ast.Ident:
		// We don't track constants within the package, so this isn't know to us now
		return constant.MakeUnknown()

	case *ast.BasicLit:
		v, err := basicLit(value)
		if err != nil {
			p.errf(value.Pos(), "Unable to parse the basic literal: %v", err)
			return constant.MakeUnknown()
		} else {
			return constant.Make(v)
		}

	case *ast.SelectorExpr:
		pkg, obj := pkgObj(p.names[file.Pkg].Files[file], value)
		if pkg != "" {
			if v, found := runtimeconstants.Get(pkg, obj); found {
				return constant.Make(v)
			}
		}

	case *ast.BinaryExpr:
		x := p.parseConstantValue(file, value.X)
		y := p.parseConstantValue(file, value.Y)
		return constant.BinaryOp(x, value.Op, y)

	case *ast.UnaryExpr:
		x := p.parseConstantValue(file, value.X)
		fmt.Println("Uniary op", value.Op, x)
		return constant.UnaryOp(value.Op, x, 0)

	default:
		p.errf(value.Pos(), "Unable to parse constant value, unknown data type: %v", reflect.TypeOf(value))
	}

	return constant.MakeUnknown()
}

func basicLit(value *ast.BasicLit) (any, error) {
	switch value.Kind {
	case token.IDENT:
		return value.Value, nil
	case token.INT:
		return strconv.ParseInt(value.Value, 10, 64)
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
