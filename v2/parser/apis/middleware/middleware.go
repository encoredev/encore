package middleware

import (
	"fmt"
	"go/ast"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	schema2 "encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/apis/directive"
	"encr.dev/v2/parser/apis/selector"
	"encr.dev/v2/parser/internal/utils"
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
	ok := directive.Validate(d.Errs, d.Dir, directive.ValidateSpec{
		AllowedOptions: []string{"global"},
		AllowedFields:  []string{"target"},
		ValidateOption: nil,
		ValidateField: func(errs *perr.List, f directive.Field) (ok bool) {
			switch f.Key {
			case "target":
				parts := f.List()
				for _, p := range parts {
					sel, ok := selector.Parse(errs, f, p)
					if !ok {
						return false
					}

					switch sel.Type {
					case selector.Tag, selector.All:
					default:
						errs.Add(errInvalidSelectorType(sel.Type).AtGoNode(f))
						return false
					}
					mw.Target.Add(sel)
				}
			}
			return true
		},
		ValidateTag: nil,
	})
	if !ok {
		return mw
	}

	sig := decl.Type
	numParams := len(sig.Params)

	// Validate the input
	if numParams < 2 {
		d.Errs.Add(errWrongNumberParams(numParams).AtGoNode(sig.AST.Params))
		return mw
	} else if numParams > 2 {
		d.Errs.Add(errWrongNumberParams(numParams).AtGoNode(sig.AST.Params))
	}

	numResults := len(sig.Results)
	if numResults < 1 {
		d.Errs.Add(errWrongNumberResults(numResults).AtGoNode(sig.AST.Results))
		return mw
	} else if numResults > 1 {
		d.Errs.Add(errWrongNumberResults(numResults).AtGoNode(sig.AST.Results))
	}

	if !schemautil.IsNamed(sig.Params[0].Type, "encore.dev/middleware", "Request") {
		d.Errs.Add(
			errInvalidFirstParam.
				AtGoNode(sig.Params[0].AST, errors.AsError(fmt.Sprintf("got %s", utils.PrettyPrint(sig.Params[0].Type.ASTExpr())))),
		)
	}
	if !schemautil.IsNamed(sig.Params[1].Type, "encore.dev/middleware", "Next") {
		d.Errs.Add(
			errInvalidSecondParam.
				AtGoNode(sig.Params[1].AST, errors.AsError(fmt.Sprintf("got %s", utils.PrettyPrint(sig.Params[1].Type.ASTExpr())))),
		)
	}
	if !schemautil.IsNamed(sig.Results[0].Type, "encore.dev/middleware", "Response") {
		d.Errs.Add(
			errInvalidReturnType.
				AtGoNode(sig.Results[0].AST, errors.AsError(fmt.Sprintf("got %s", utils.PrettyPrint(sig.Results[0].Type.ASTExpr())))),
		)
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
