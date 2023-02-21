package apis

import (
	"go/ast"

	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/parser2/internal/schema/schemautil"
	"encr.dev/pkg/option"
)

// AuthHandler describes an Encore authentication handler.
type AuthHandler struct {
	Decl *schema.FuncDecl
	Doc  string

	// Param is the auth parameters.
	// It's either a builtin string for token-based authentication,
	// or a named struct type for complex auth parameters.
	Param schema.Type

	// Recv is the type the auth handler is defined as a method on, if any.
	Recv option.Option[*schema.Receiver]

	// AuthData is the custom auth data type the app specifies
	// as part of the returns from the auth handler, if any.
	AuthData option.Option[*schema.TypeDeclRef]
}

// parseAuthHandler parses the auth handler in the provided declaration.
func (p *Parser) parseAuthHandler(file *pkginfo.File, fd *ast.FuncDecl, dir *authHandlerDirective, doc string) *AuthHandler {
	decl := p.schema.ParseFuncDecl(file, fd)

	ah := &AuthHandler{
		Decl: decl,
		Doc:  doc,
		Recv: decl.Recv,
	}

	const sigHint = `
	hint: valid signatures are:
	- func(ctx context.Context, p *Params) (auth.UID, error)
	- func(ctx context.Context, p *Params) (auth.UID, *UserData, error)
	- func(ctx context.Context, token string) (auth.UID, error)
	- func(ctx context.Context, token string) (auth.UID, *UserData, error)

	note: *Params and *UserData are custom data types you define`

	sig := decl.Type
	numParams := len(sig.Params)

	// Validate the input
	if numParams < 2 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid auth handler signature (too few parameters)"+sigHint)
		return ah
	} else if numParams > 2 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid auth handler signature (too many parameters)"+sigHint)
	}

	numResults := len(sig.Results)
	if numResults < 2 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid auth handler signature (too few results)"+sigHint)
		return ah
	} else if numResults > 3 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid auth handler signature (too many results)"+sigHint)
	}

	// First param should always be context.Context
	ctxParam := sig.Params[0]
	if !schemautil.IsNamed(ctxParam.Type, "context", "Context") {
		p.c.Errs.Add(ctxParam.AST.Pos(), "first parameter must be of type context.Context"+sigHint)
	}

	ah.Param = sig.Params[1].Type

	// Second param should be string, or a pointer to a named struct
	{
		param := ah.Param
		if schemautil.IsBuiltinKind(param, schema.String) {
			// All good
		} else if _, ok := schemautil.ResolveNamedStruct(param, true); !ok {
			p.c.Errs.Add(param.ASTExpr().Pos(), "second parameter must be a string, or a pointer to a named struct"+sigHint)
		}
	}

	// First result must be auth.UID
	if uid := sig.Results[0]; !schemautil.IsBuiltinKind(uid.Type, schema.UserID) {
		p.c.Errs.Add(uid.AST.Pos(), "first result must be of type auth.UID"+sigHint)
	}

	// If we have three results, the second one must be a pointer to a named struct.
	if numResults > 2 {
		if ref, ok := schemautil.ResolveNamedStruct(sig.Results[1].Type, true); !ok {
			p.c.Errs.Add(sig.Results[1].AST.Pos(), "second result must be a pointer to a named struct"+sigHint)
		} else {
			ah.AuthData = option.Some(ref)
		}
	}

	return ah
}
