package parseutil

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/internal/perr"
)

const resourceNameMaxLength int = 63

type resourceNameSpec struct {
	regexp         *regexp.Regexp
	invalidNameErr func(fset *token.FileSet, node ast.Node, resourceType, paramName, name string) error
	reservedErr    func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error
}

var KebabName = resourceNameSpec{
	regexp:         regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
	invalidNameErr: srcerrors.ResourceNameNotKebabCase,
	reservedErr: func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error {
		return srcerrors.ResourceNameReserved(fset, node, resourceType, paramName, name, reservedPrefix, false)
	},
}

var SnakeName = resourceNameSpec{
	regexp:         regexp.MustCompile(`^[a-z]([_a-z0-9]*[a-z0-9])?$`),
	invalidNameErr: srcerrors.ResourceNameNotSnakeCase,
	reservedErr: func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error {
		return srcerrors.ResourceNameReserved(fset, node, resourceType, paramName, name, reservedPrefix, true)
	},
}

// ParseResourceName checks the given node is a string literal
// and that it conforms to the given spec.
//
// If an error is encountered, it will report a parse error and return an empty string
// otherwise it will return the parsed resource name
func ParseResourceName(errs *perr.List, resourceType string, paramName string, node ast.Expr, nameSpec resourceNameSpec, reservedPrefix string) string {
	name, ok := literals.ParseString(node)
	if !ok {
		//errs.errInSrc(srcerrors.ResourceNameNotStringLiteral(p.fset, node, resourceType, paramName))
		errs.Add(node.Pos(), "resource name must be a string literal")
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > resourceNameMaxLength {
		//p.errInSrc(srcerrors.ResourceNameWrongLength(p.fset, node, resourceType, paramName, name))
		errs.Add(node.Pos(), "resource name too long (must be at most 63 characters)")
		return ""
	}

	if !nameSpec.regexp.MatchString(name) {
		//p.errInSrc(nameSpec.invalidNameErr(p.fset, node, resourceType, paramName, name))
		return ""
	} else if reservedPrefix != "" && strings.HasPrefix(name, reservedPrefix) {
		//p.errInSrc(nameSpec.reservedErr(p.fset, node, resourceType, paramName, name, reservedPrefix))
		errs.Add(node.Pos(), "resource name is reserved")
		return ""
	}

	return name
}
