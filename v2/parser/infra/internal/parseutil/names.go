package parseutil

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/errors"
	"encr.dev/pkg/idents"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/parser/infra/internal/literals"
)

const resourceNameMaxLength int = 63

type resourceNameSpec struct {
	regexp         *regexp.Regexp
	errDetails     func(resourceName, paramName string) string
	invalidNameErr func(node ast.Node, resourceName, paramName, name string) errors.Template
	reservedErr    func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error
}

var KebabName = resourceNameSpec{
	regexp:     regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`),
	errDetails: resourceNameHelpKebabCase,
	invalidNameErr: func(node ast.Node, resourceType, paramName, name string) errors.Template {
		err := errResourceNameNotCorrectFormat(resourceType, paramName, "kebab-case").
			WithDetails(resourceNameHelpKebabCase(resourceType, paramName)).
			AtGoNode(node, errors.AsError(fmt.Sprintf("try %s?", idents.GenerateSuggestion(name, idents.KebabCase))))

		return err
	},
	reservedErr: func(fset *token.FileSet, node ast.Node, resourceType, paramName, name, reservedPrefix string) error {
		return srcerrors.ResourceNameReserved(fset, node, resourceType, paramName, name, reservedPrefix, false)
	},
}

var SnakeName = resourceNameSpec{
	regexp:     regexp.MustCompile(`^[a-z]([_a-z0-9]*[a-z0-9])?$`),
	errDetails: resourceNameHelpSnakeCase,
	invalidNameErr: func(node ast.Node, resourceType, paramName, name string) errors.Template {
		err := errResourceNameNotCorrectFormat(resourceType, paramName, "snake_case").
			WithDetails(resourceNameHelpSnakeCase(resourceType, paramName)).
			AtGoNode(node, errors.AsError(fmt.Sprintf("try %s?", idents.GenerateSuggestion(name, idents.SnakeCase))))

		return err
	},
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
		errs.Add(
			errResourceNameMustBeStringLiteral(resourceType, paramName).
				WithDetails(nameSpec.errDetails(resourceType, paramName)).
				AtGoNode(node, errors.AsError(fmt.Sprintf("was given %s", NodeType(node)))),
		)
		return ""
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > resourceNameMaxLength {
		errs.Add(
			errResourceNameInvalidLength(resourceType, paramName).
				WithDetails(nameSpec.errDetails(resourceType, paramName)).
				AtGoNode(node, errors.AsError(fmt.Sprintf("is %d long", len(name)))),
		)
		return ""
	}

	if !nameSpec.regexp.MatchString(name) {
		errs.Add(nameSpec.invalidNameErr(node, resourceType, paramName, name))
		return ""
	} else if reservedPrefix != "" && strings.HasPrefix(name, reservedPrefix) {
		errs.Add(
			errResourceNameReserved(resourceType, paramName, name, reservedPrefix).
				WithDetails(nameSpec.errDetails(resourceType, paramName)).
				AtGoNode(node),
		)
		return ""
	}

	return name
}

func resourceNameHelpKebabCase(resourceName string, paramName string) string {
	return fmt.Sprintf("%s %s's must be defined as string literals, "+
		"be between 1 and 63 characters long, and defined in \"kebab-case\", meaning it must start with a letter, end with a letter "+
		"or number and only contain lower case letters, numbers and dashes.",
		resourceName, paramName,
	)
}

func resourceNameHelpSnakeCase(resourceName string, paramName string) string {
	return fmt.Sprintf("%s %s's must be defined as string literals, "+
		"be between 1 and 63 characters long, and defined in \"snake_case\", meaning it must start with a letter, end with a letter "+
		"or number and only contain lower case letters, numbers and underscores.",
		resourceName, paramName,
	)
}
