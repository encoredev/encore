package middleware

import (
	"fmt"
	"go/ast"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	schema2 "encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/apis/directive"
	"encr.dev/v2/parser/apis/selector"
)

// Middleware describes an Encore middleware.
type Middleware struct {
	Decl *schema2.FuncDecl
	Doc  string
	File *pkginfo.File // file it's declared in

	// Global specifies whether the middleware is global or service-local.
	Global bool

	// Target specifies the set of API endpoints the middleware applies to.
	Target selector.Set

	// Recv is the type the middleware is defined as a method on, if any.
	Recv option.Option[*schema2.Receiver]
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema2.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses the middleware in the provided declaration.
func Parse(d ParseData) *Middleware {
	decl := d.Schema.ParseFuncDecl(d.File, d.Func)

	mw := &Middleware{
		Decl:   decl,
		Doc:    d.Doc,
		File:   d.File,
		Recv:   decl.Recv,
		Global: d.Dir.HasOption("global"),
	}
	err := directive.Validate(d.Dir, directive.ValidateSpec{
		AllowedOptions: []string{"global"},
		AllowedFields:  []string{"target"},
		ValidateOption: nil,
		ValidateField: func(f directive.Field) (err error) {
			switch f.Key {
			case "target":
				parts := f.List()
				for _, p := range parts {
					sel, err := selector.Parse(p)
					if err != nil {
						return fmt.Errorf("invalid selector format %q: %v", p, err)
					}

					switch sel.Type {
					case selector.Tag, selector.All:
					default:
						return fmt.Errorf("middleware target only supports tags as selectors (got '%s')", sel.Type)
					}
					mw.Target.Add(sel)
				}
			}
			return err
		},
		ValidateTag: nil,
	})
	if err != nil {
		d.Errs.Addf(d.Dir.AST.Pos(), "invalid encore:middleware directive: %v", err)
		return mw
	}

	const sigHint = `
	hint: middleware must have the signature:
	func(req middleware.Request, next middleware.Next) middleware.Response`

	sig := decl.Type
	numParams := len(sig.Params)

	// Validate the input
	if numParams < 2 {
		d.Errs.AddPos(sig.AST.Pos(), "invalid middleware signature (too few parameters)"+sigHint)
		return mw
	} else if numParams > 2 {
		d.Errs.AddPos(sig.AST.Pos(), "invalid middleware signature (too many parameters)"+sigHint)
	}

	numResults := len(sig.Results)
	if numResults < 1 {
		d.Errs.AddPos(sig.AST.Pos(), "invalid middleware signature (too few results)"+sigHint)
		return mw
	} else if numResults > 1 {
		d.Errs.AddPos(sig.AST.Pos(), "invalid middleware signature (too many results)"+sigHint)
	}

	if !schemautil.IsNamed(sig.Params[0].Type, "encore.dev/middleware", "Request") {
		d.Errs.AddPos(sig.Params[0].AST.Pos(), "first parameter type must be middleware.Request"+sigHint)
	}
	if !schemautil.IsNamed(sig.Params[1].Type, "encore.dev/middleware", "Next") {
		d.Errs.AddPos(sig.Params[0].AST.Pos(), "second parameter type must be middleware.Next"+sigHint)
	}
	if !schemautil.IsNamed(sig.Results[0].Type, "encore.dev/middleware", "Response") {
		d.Errs.AddPos(sig.Params[0].AST.Pos(), "return type must be middleware.Response"+sigHint)
	}

	return mw
}

// Sort sorts the middleware to ensure they execute in deterministic order.
func Sort(mws []*Middleware) {
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
