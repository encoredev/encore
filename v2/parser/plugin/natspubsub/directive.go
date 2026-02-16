package natspubsub

import (
	"fmt"
	"go/ast"

	"encr.dev/v2/parser/apis/directive"
)

func init() {
	directive.RegisterDirectiveParser("pubsub", parsePubSub)
}

// parsePubSub handles "//encore:pubsub <subject>" above a FuncDecl.
func parsePubSub(d *directive.Directive, decl *ast.FuncDecl) error {
	if len(d.Options) != 1 {
		return fmt.Errorf("pubsub directive requires exactly one subject argument, got %d", len(d.Options))
	}

	// Validate handler signature: func(context.Context, *T) error
	if len(decl.Type.Params.List) != 2 {
		return fmt.Errorf("pubsub: handler must have two parameters (ctx, *Event)")
	}
	if decl.Type.Results == nil || len(decl.Type.Results.List) != 1 {
		return fmt.Errorf("pubsub: handler must return exactly one value (error)")
	}
	if ident, ok := decl.Type.Results.List[0].Type.(*ast.Ident); !ok || ident.Name != "error" {
		return fmt.Errorf("pubsub: handler must return error")
	}

	return nil
}
