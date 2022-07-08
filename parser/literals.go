package parser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/runtimeconstants"
)

var noOpCasts = map[string][]string{
	"encore.dev/cron": {"Duration"},
	"time":            {"Duration"},
}

// litString will return the string value of a given node
//
// If the given node isn't a string literal, it will return an empty string and false
func litString(node ast.Node) (string, bool) {
	if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return lit.Value[1 : len(lit.Value)-1], true
	}
	return "", false
}

// parseStructLit parses struct literal and returns a LiterialStruct object
//
// If there is a nested struct literal, then it's values will be nested in a dot syntax; i.e. `parentStructFieldName.childStructFieldName`
func (p *parser) parseStructLit(file *est.File, expectedType string, node ast.Expr) (lit *LiteralStruct, ok bool) {
	cl, ok := node.(*ast.CompositeLit)
	if !ok {
		p.errf(node.Pos(), "Expected a literal instance of `%s`, got %s.", expectedType, prettyPrint(node))
		return nil, false
	}

	lit = &LiteralStruct{
		ast:            node,
		constantFields: make(map[string]constant.Value),
		allFields:      make(map[string]ast.Expr),
		childStructs:   make(map[string]*LiteralStruct),
	}
	ok = true

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
			var subStruct *ast.CompositeLit
			switch value := elem.Value.(type) {
			case *ast.UnaryExpr:
				if value.Op == token.AND {
					if compositeLiteral, ok := value.X.(*ast.CompositeLit); ok {
						subStruct = compositeLiteral
					}
				}

			case *ast.CompositeLit:
				subStruct = value
			}

			if subStruct != nil {
				subLit, subOk := p.parseStructLit(file, "struct", subStruct)
				ok = ok && subOk
				lit.childStructs[ident.Name] = subLit
			} else if valueIdent, ok := elem.Value.(*ast.Ident); ok && valueIdent.Name == "nil" {
				// no-op for nil's
			} else {
				// Parse the value
				lit.allFields[ident.Name] = elem.Value
				value := p.parseConstantValue(file, elem.Value)
				if value.Kind() != constant.Unknown {
					lit.constantFields[ident.Name] = value
				}
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
		switch value.Name {
		case "true":
			return constant.MakeBool(true)
		case "false":
			return constant.MakeBool(false)
		default:
			return constant.MakeUnknown()
		}

	case *ast.BasicLit:
		v, err := basicLit(value)
		if err != nil {
			p.errf(value.Pos(), "Unable to parse the basic literal: %v", err)
			return constant.MakeUnknown()
		} else {
			return v
		}

	case *ast.SelectorExpr:
		pkg, obj := pkgObj(p.names[file.Pkg].Files[file], value)
		if pkg != "" {
			if v, found := runtimeconstants.Get(pkg, obj); found {
				return constant.Make(v)
			}
		}

		return constant.MakeUnknown()

	case *ast.BinaryExpr:
		lhs := p.parseConstantValue(file, value.X)
		rhs := p.parseConstantValue(file, value.Y)
		if lhs.Kind() == constant.Unknown || rhs.Kind() == constant.Unknown {
			return constant.MakeUnknown()
		}

		switch value.Op {
		case token.MUL, token.ADD, token.SUB, token.REM, token.AND, token.OR, token.XOR, token.AND_NOT:
			return constant.BinaryOp(lhs, value.Op, rhs)
		case token.QUO:
			// constant.BinaryOp panics when dividing by zero
			if floatValue, _ := constant.Float64Val(constant.ToFloat(rhs)); floatValue <= 0.000000001 && floatValue >= -0.000000001 {
				fmt.Printf("this is a float! %v\n", floatValue)
				p.errf(value.Pos(), "cannot divide by zero")
				return constant.MakeUnknown()
			}

			fmt.Println("Divide")

			return constant.BinaryOp(lhs, value.Op, rhs)
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			return constant.MakeBool(constant.Compare(lhs, value.Op, rhs))

		case token.SHL, token.SHR:
			shiftValue, ok := constant.Uint64Val(constant.ToInt(rhs))
			if !ok {
				p.errf(value.Pos(), "shift count must be an unsigned integer")
			}
			return constant.Shift(lhs, value.Op, uint(shiftValue))

		default:
			p.errf(value.Pos(), "%s is an unsupported operation here", value.Op)
			return constant.MakeUnknown()
		}

	case *ast.UnaryExpr:
		x := p.parseConstantValue(file, value.X)
		return constant.UnaryOp(value.Op, x, 0)

	case *ast.CallExpr:
		// We allow casts like "time.Duration(143)" or "cron.Duration(143)"
		// so we transparently go through them
		if sel, ok := value.Fun.(*ast.SelectorExpr); ok && len(value.Args) == 1 {
			pkg, obj := pkgObj(p.names[file.Pkg].Files[file], sel)
			if pkgFuncs, found := noOpCasts[pkg]; found {
				for _, allowed := range pkgFuncs {
					if allowed == obj {
						return p.parseConstantValue(file, value.Args[0])
					}
				}
			}
		}
		p.errf(value.Pos(), "You can not call a function here, only constant values are supported")
		return constant.MakeUnknown()

	case *ast.ParenExpr:
		return p.parseConstantValue(file, value.X)

	default:
		p.errf(value.Pos(), "Unable to parse constant value, unknown data type: %v", reflect.TypeOf(value))
		return constant.MakeUnknown()
	}
}

func basicLit(value *ast.BasicLit) (constant.Value, error) {
	switch value.Kind {
	case token.IDENT:
		return constant.MakeString(value.Value), nil
	case token.INT:
		v, err := strconv.ParseInt(value.Value, 10, 64)
		if err != nil {
			return constant.MakeUnknown(), err
		}
		return constant.MakeInt64(v), nil
	case token.FLOAT:
		v, err := strconv.ParseFloat(value.Value, 64)
		if err != nil {
			return constant.MakeUnknown(), err
		}
		return constant.MakeFloat64(v), nil
	case token.CHAR:
		c, _, _, err := strconv.UnquoteChar(value.Value, value.Value[0])
		return constant.MakeFromBytes([]byte{byte(c)}), err
	case token.STRING:
		return constant.MakeString(value.Value[1 : len(value.Value)-1]), nil
	default:
		return nil, errors.New("unsupported literal type")
	}
}

// LiteralStruct represents a struct literal at compile time
type LiteralStruct struct {
	ast            ast.Expr                  // The AST node which presents the literal
	constantFields map[string]constant.Value // All found constant expressions
	allFields      map[string]ast.Expr       // All field expressions (constant or otherwise)
	childStructs   map[string]*LiteralStruct // Any child struct literals
}

// FullyConstant returns true if every value in this struct and the child structs fully known as compile time
// as a constant value
func (l *LiteralStruct) FullyConstant() bool {
	for _, sub := range l.childStructs {
		if !sub.FullyConstant() {
			return false
		}
	}
	return len(l.constantFields) == len(l.allFields)
}

// DynamicFields returns the names of the fields and ast.Expr that are not constant
//
// Child structs will be included with the field name prefixed with the struct name;
// i.e. `parentField.childField`
func (l *LiteralStruct) DynamicFields() map[string]ast.Expr {
	fields := make(map[string]ast.Expr)
	for name, expr := range l.allFields {
		if _, found := l.constantFields[name]; !found {
			fields[name] = expr
		}
	}

	for name, sub := range l.childStructs {
		for k, v := range sub.DynamicFields() {
			fields[name+"."+k] = v
		}
	}

	return fields
}

// IsSet returns true if the given field is set in this struct
//
// You can reference a child struct field with `.`; i.e. `parent.child`
func (l *LiteralStruct) IsSet(fieldName string) bool {
	// Recurse into child fields
	before, after, found := strings.Cut(fieldName, ".")
	if found {
		if child, found := l.childStructs[before]; found {
			return child.IsSet(after)
		} else {
			return false
		}
	}

	_, found = l.allFields[fieldName]
	return found
}

// Pos returns the position of the field in the source code
//
// If the field is not found, the closest position to where
// the field should have been will be returned
//
// You can reference a child struct field with `.`; i.e. `parent.child`
func (l *LiteralStruct) Pos(fieldName string) token.Pos {
	before, after, found := strings.Cut(fieldName, ".")
	if found {
		if child, found := l.childStructs[before]; found {
			return child.Pos(after)
		} else {
			return l.ast.Pos()
		}
	}

	value, found := l.allFields[fieldName]
	if found {
		return value.Pos()
	} else {
		return l.ast.Pos()
	}
}

// Expr returns ast.Expr for the given field name.
//
// If the field is known, it returns the ast.Expr
// If the field is not known, it returns nil
//
// You can reference a child struct field with `.`; i.e. `parent.child`
func (l *LiteralStruct) Expr(fieldName string) ast.Expr {
	// Recurse into child fields
	before, after, found := strings.Cut(fieldName, ".")
	if found {
		if child, found := l.childStructs[before]; found {
			return child.Expr(after)
		} else {
			return nil
		}
	}

	value, found := l.allFields[fieldName]
	if found {
		return value
	} else {
		return nil
	}
}

// Value returns the value of the field as a constant.Value. If the field is not constant or
// does not exist, an unknown value will be returned
//
// You can reference a child struct field with `.`; i.e. `parent.child`
func (l *LiteralStruct) Value(fieldName string) constant.Value {
	// Recurse into child fields
	before, after, found := strings.Cut(fieldName, ".")
	if found {
		if child, found := l.childStructs[before]; found {
			return child.Value(after)
		} else {
			return constant.MakeUnknown()
		}
	}

	value, found := l.constantFields[fieldName]
	if !found {
		return constant.MakeUnknown()
	}
	return value
}

// Int64 returns the value of the field as an int64
//
// This function will convert other number types into an Int64, but will not convert strings.
// If after conversion the value is 0, the defaultValue will be returned
//
// You can reference a child struct field with `.`; i.e. `parent.child`
func (l *LiteralStruct) Int64(fieldName string, defaultValue int64) int64 {
	realValue, ok := constant.Int64Val(constant.ToInt(l.Value(fieldName)))
	if !ok || realValue == 0 {
		return defaultValue
	}
	return realValue
}

// Str returns the value of the field as an string
//
// This function will convert all types to a string
// If after conversion the value is "", the defaultValue will be returned
//
// You can reference a child struct field with `.`; i.e. `parent.child`
func (l *LiteralStruct) Str(fieldName string, defaultValue string) string {
	value := l.Value(fieldName)

	str := value.ExactString()
	if value.Kind() == constant.String || value.Kind() == constant.Unknown {
		str = constant.StringVal(value)

	}

	if str == "" {
		return defaultValue
	} else {
		return str
	}
}
