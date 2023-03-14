package authhandler

import (
	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/parser/resource/usage"
)

func ResolveAuthHandlerUsage(errs *perr.List, expr usage.Expr, handler *AuthHandler) usage.Usage {
	switch expr := expr.(type) {
	case *usage.FuncCall:
		if expr.DeclaredIn().Pkg != handler.Package() {
			errs.Add(
				errCannotCallFromAnotherPackage.
					AtGoNode(expr, errors.AsError("called here")).
					AtGoNode(handler.Decl.AST.Name, errors.AsHelp("auth handler defined here")),
			)
		}
	default:
		errs.Add(
			errInvalidReference.
				AtGoNode(expr, errors.AsError("referenced here")).
				AtGoNode(handler.Decl.AST.Name, errors.AsHelp("auth handler defined here")),
		)
	}

	return nil
}
