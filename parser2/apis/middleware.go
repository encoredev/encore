package apis

import (
	"go/ast"

	"golang.org/x/exp/slices"

	"encr.dev/parser2/apis/selector"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/parser2/internal/schema/schemautil"
	"encr.dev/pkg/option"
)

// Middleware describes an Encore middleware.
type Middleware struct {
	Decl *schema.FuncDecl
	Doc  string

	// Global specifies whether the middleware is global or service-local.
	Global bool

	// Target specifies the set of API endpoints the middleware applies to.
	Target selector.Set

	// Recv is the type the middleware is defined as a method on, if any.
	Recv option.Option[*schema.Receiver]
}

// parseMiddleware parses the auth handler in the provided declaration.
func (p *Parser) parseMiddleware(file *pkginfo.File, fd *ast.FuncDecl, dir *middlewareDirective, doc string) *Middleware {
	decl := p.schema.ParseFuncDecl(file, fd)

	mw := &Middleware{
		Decl:   decl,
		Doc:    doc,
		Recv:   decl.Recv,
		Global: dir.Global,
		Target: dir.Target,
	}

	const sigHint = `
	hint: middleware must have the signature:
	func(req middleware.Request, next middleware.Next) middleware.Response`

	sig := decl.Type
	numParams := len(sig.Params)

	// Validate the input
	if numParams < 2 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid middleware signature (too few parameters)"+sigHint)
		return mw
	} else if numParams > 2 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid middleware signature (too many parameters)"+sigHint)
	}

	numResults := len(sig.Results)
	if numResults < 1 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid middleware signature (too few results)"+sigHint)
		return mw
	} else if numResults > 1 {
		p.c.Errs.Add(sig.AST.Pos(), "invalid middleware signature (too many results)"+sigHint)
	}

	if !schemautil.IsNamed(sig.Params[0].Type, "encore.dev/middleware", "Request") {
		p.c.Errs.Add(sig.Params[0].AST.Pos(), "first parameter type must be middleware.Request"+sigHint)
	}
	if !schemautil.IsNamed(sig.Params[1].Type, "encore.dev/middleware", "Next") {
		p.c.Errs.Add(sig.Params[0].AST.Pos(), "second parameter type must be middleware.Next"+sigHint)
	}
	if !schemautil.IsNamed(sig.Results[0].Type, "encore.dev/middleware", "Response") {
		p.c.Errs.Add(sig.Params[0].AST.Pos(), "return type must be middleware.Response"+sigHint)
	}

	return mw
}

// sortMiddleware sorts the middleware to ensure they
// execute in deterministic order.
func (p *Parser) sortMiddleware(mws []*Middleware) {
	sortFn := func(a, b *Middleware) bool {
		// Globals come first
		if a.Global != b.Global {
			return a.Global
		}

		// Then sort by package path
		aPkg, bPkg := a.Decl.File.Pkg, b.Decl.File.Pkg
		if aPkg.ImportPath != bPkg.ImportPath {
			return aPkg.ImportPath < bPkg.ImportPath
		}

		// Then sort by file name
		if aFile, bFile := a.Decl.File.Name, b.Decl.File.Name; aFile != bFile {
			return aFile < bFile
		}

		// Sort by declaration order within the file
		return a.Decl.AST.Pos() < b.Decl.AST.Pos()
	}

	slices.SortStableFunc(mws, sortFn)
}
