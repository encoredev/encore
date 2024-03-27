package middleware

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"slices"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/directive"
	"encr.dev/v2/parser/apis/selector"
	"encr.dev/v2/parser/internal/utils"
	"encr.dev/v2/parser/resource"
)

// Middleware describes an Encore middleware.
type Middleware struct {
	Decl *schema.FuncDecl
	Doc  string
	File *pkginfo.File // file it's declared in

	// Global specifies whether the middleware is global or service-local.
	Global bool

	// Target specifies the set of API endpoints the middleware applies to.
	Target selector.Set

	// Recv is the type the middleware is defined as a method on, if any.
	Recv option.Option[*schema.Receiver]
}

// ID returns a unique id for this specific middleware.
func (mw *Middleware) ID() string {
	return mw.Decl.File.Pkg.ImportPath.String() + "." + mw.Decl.Name
}

func (mw *Middleware) Kind() resource.Kind       { return resource.Middleware }
func (mw *Middleware) Package() *pkginfo.Package { return mw.File.Pkg }
func (mw *Middleware) Pos() token.Pos            { return mw.Decl.AST.Pos() }
func (mw *Middleware) End() token.Pos            { return mw.Decl.AST.End() }
func (mw *Middleware) SortKey() string {
	return mw.Decl.File.Pkg.ImportPath.String() + "." + mw.Decl.Name
}

type ParseData struct {
	Errs   *perr.List
	Schema *schema.Parser

	File *pkginfo.File
	Func *ast.FuncDecl
	Dir  *directive.Directive
	Doc  string
}

// Parse parses the middleware in the provided declaration.
// It may return nil on errors.
func Parse(d ParseData) *Middleware {
	decl, ok := d.Schema.ParseFuncDecl(d.File, d.Func)
	if !ok {
		return nil
	}

	mw := &Middleware{
		Decl:   decl,
		Doc:    d.Doc,
		File:   d.File,
		Recv:   decl.Recv,
		Global: d.Dir.HasOption("global"),
	}
	ok = directive.Validate(d.Errs, d.Dir, directive.ValidateSpec{
		AllowedOptions: []string{"global"},
		AllowedFields:  []string{"target"},
		ValidateOption: nil,
		ValidateField: func(errs *perr.List, f directive.Field) (ok bool) {
			switch f.Key {
			case "target":
				parts := f.List()
				for _, p := range parts {
					sel, ok := selector.Parse(errs, f.Pos()+7, p) // + 7 for "target="
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
	sortFn := func(a, b *Middleware) int {
		// Globals come first
		if a.Global != b.Global {
			if a.Global {
				return -1
			} else {
				return 1
			}
		}

		// Then sort by package path
		aPkg, bPkg := a.Decl.File.Pkg, b.Decl.File.Pkg
		if n := cmp.Compare(aPkg.ImportPath, bPkg.ImportPath); n != 0 {
			return n
		}

		// Then sort by file name
		if n := cmp.Compare(a.Decl.File.Name, b.Decl.File.Name); n != 0 {
			return n
		}

		// Sort by declaration order within the file
		return cmp.Compare(a.Decl.AST.Pos(), b.Decl.AST.Pos())
	}

	slices.SortStableFunc(mws, sortFn)
}
