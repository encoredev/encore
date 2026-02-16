package natspubsub

import (
	"fmt"
	"go/ast"
	"strings"

	"encr.dev/v2/parser/apis/directive"
)

func init() {
	directive.RegisterDirectiveParser("pubsub", parsePubSub)
}

// parsePubSub handles "//encore:pubsub <subject>" above a FuncDecl.
func parsePubSub(d *directive.Directive, decl *ast.FuncDecl) error {
	if d == nil {
		return fmt.Errorf("pubsub directive is nil")
	}
	if decl == nil || decl.Type == nil || decl.Type.Params == nil {
		return fmt.Errorf("pubsub directive must annotate a function declaration")
	}
	if len(d.Fields) > 0 || len(d.Tags) > 0 {
		return fmt.Errorf("pubsub directive accepts only one positional subject argument")
	}
	if len(d.Options) != 1 {
		return fmt.Errorf("pubsub directive requires exactly one subject argument, got %d", len(d.Options))
	}
	if err := validateNATSSubject(d.Options[0].Value); err != nil {
		return fmt.Errorf("invalid pubsub subject %q: %w", d.Options[0].Value, err)
	}

	// Validate handler signature: func(context.Context, *T) error
	if len(decl.Type.Params.List) != 2 {
		return fmt.Errorf("pubsub: handler must have two parameters (context.Context, *Event)")
	}
	if !isContextParam(decl.Type.Params.List[0].Type) {
		return fmt.Errorf("pubsub: first handler parameter must be context.Context")
	}
	if _, ok := decl.Type.Params.List[1].Type.(*ast.StarExpr); !ok {
		return fmt.Errorf("pubsub: second handler parameter must be a pointer type (*Event)")
	}
	if decl.Type.Results == nil || len(decl.Type.Results.List) != 1 {
		return fmt.Errorf("pubsub: handler must return exactly one value (error)")
	}
	if ident, ok := decl.Type.Results.List[0].Type.(*ast.Ident); !ok || ident.Name != "error" {
		return fmt.Errorf("pubsub: handler must return error")
	}

	return nil
}

func isContextParam(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "context" && sel.Sel != nil && sel.Sel.Name == "Context"
}

func validateNATSSubject(subject string) error {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}

	tokens := strings.Split(subject, ".")
	for i, tok := range tokens {
		if tok == "" {
			return fmt.Errorf("subject cannot contain empty tokens")
		}
		if strings.ContainsAny(tok, " \t\n\r") {
			return fmt.Errorf("subject token %q contains whitespace", tok)
		}
		if strings.Contains(tok, ">") && tok != ">" {
			return fmt.Errorf("token %q contains invalid > wildcard usage", tok)
		}
		if strings.Contains(tok, "*") && tok != "*" {
			return fmt.Errorf("token %q contains invalid * wildcard usage", tok)
		}
		if tok == ">" && i != len(tokens)-1 {
			return fmt.Errorf("> wildcard is only allowed as the final token")
		}
	}
	return nil
}
