package srcerrors

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	cueerrors "cuelang.org/go/cue/errors"

	"encr.dev/pkg/errinsrc"
	. "encr.dev/pkg/errinsrc/internal"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

func handleCUEError(err error, pathPrefix string, param ErrParams) error {
	if err == nil {
		return nil
	}

	toReturn := make(errinsrc.List, 0, 1)

	if param.Detail == "" {
		param.Detail = "For more information on CUE and this error, see https://cuelang.org/docs/"
	}

	for _, e := range cueerrors.Errors(err) {
		param.Summary = e.Error()
		param.Cause = e
		param.Locations = LocationsFromCueError(e, pathPrefix)
		toReturn = append(toReturn, errinsrc.New(param, false))
	}

	switch len(toReturn) {
	case 0:
		return nil
	case 1:
		return toReturn[0]
	default:
		return toReturn
	}
}

// Converts a node to a string which looks like the original go code.
// such as a ast.SelectorExpr will become "foo.Blah"
//
// It's not intended to be an exact representation, but rather a helperful
// representation for error messages.
func nodeAsGoSrc(node ast.Node) string {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name

	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", nodeAsGoSrc(node.X), node.Sel.Name)

	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", nodeAsGoSrc(node.X), nodeAsGoSrc(node.Index))

	case *ast.IndexListExpr:
		indices := make([]string, 0, len(node.Indices))
		for _, n := range node.Indices {
			indices = append(indices, nodeAsGoSrc(n))
		}
		return fmt.Sprintf("%s[%s]", nodeAsGoSrc(node.X), strings.Join(indices, ", "))

	case *ast.FuncLit:
		return "a function literal"

	case *ast.BasicLit:
		return node.Value

	case *ast.CallExpr:
		return fmt.Sprintf("%s(...)", nodeAsGoSrc(node.Fun))

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(node))
	}
}

// Converts a node to a string that can be used in an error message.
// such as a ast.CallExpr will return "a function call to foo.Blah"
func nodeType(node ast.Node) string {
	switch node := node.(type) {
	case *ast.Ident:
		return "an identifier"

	case *ast.SelectorExpr:
		return "an identifier"

	case *ast.IndexExpr:
		return "a identifier"

	case *ast.IndexListExpr:
		return "a identifier"

	case *ast.FuncLit:
		return "a function literal"

	case *ast.BasicLit:
		switch node.Kind {
		case token.INT:
			return "an integer literal"
		case token.FLOAT:
			return "a float literal"
		case token.IMAG:
			return "an imaginary literal"
		case token.CHAR:
			return "a character literal"
		case token.STRING:
			return "a string literal"
		default:
			return "a literal"
		}

	case *ast.CallExpr:
		return fmt.Sprintf("a function call to %s", nodeAsGoSrc(node.Fun))

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(node))
	}
}

// Converts a schema type to a string that can be used in an error message.
// such as a ast.CallExpr will return "a function call to foo.Blah"
func schemaType(typ *schema.Type) string {
	switch tt := typ.Typ.(type) {
	case *schema.Type_Named:
		return "a named type"

	case *schema.Type_Struct:
		return "a struct type"

	case *schema.Type_Map:
		return "a map type"

	case *schema.Type_List:
		return "a list type"

	case *schema.Type_Builtin:
		return fmt.Sprintf("a builtin type (%s)", tt.Builtin)

	case *schema.Type_Pointer:
		return "a pointer to " + schemaType(tt.Pointer.Base)

	case *schema.Type_TypeParameter:
		return "a type parameter"

	case *schema.Type_Config:
		return "a config value"

	default:
		return fmt.Sprintf("a %v", reflect.TypeOf(tt))
	}
}
