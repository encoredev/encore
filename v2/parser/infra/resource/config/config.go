package config

import (
	"go/ast"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

// Load represents a config load statement.
type Load struct {
	AST  *ast.CallExpr
	File *pkginfo.File

	// Type is the type of the config struct being loaded.
	// It's guaranteed to be a (possibly pointer to a) named struct type.
	Type schema.Type

	// FuncCall is the AST node that represents the config.Load expression.
	FuncCall *ast.CallExpr
}

func (*Load) Kind() resource.Kind         { return resource.ConfigLoad }
func (l *Load) Package() *pkginfo.Package { return l.File.Pkg }
func (l *Load) ASTExpr() ast.Expr         { return l.AST }

var LoadParser = &resource.Parser{
	Name: "ConfigLoad",

	InterestingImports: []paths.Pkg{"encore.dev/config"},
	Run: func(p *resource.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/config", Name: "Load"}

		spec := &parseutil.ReferenceSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parseLoad,
		}
		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseReference(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseLoad(d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs

	if len(d.Call.Args) > 0 {
		errs.Add(errInvalidLoad.AtGoNode(d.Call))
		return
	}

	// Resolve the named struct used for the config type
	ref, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		errs.Add(errInvalidConfigType.AtGoNode(d.TypeArgs[0].ASTExpr()))
		return
	}
	_ = ref

	load := &Load{
		AST:      d.Call,
		File:     d.File,
		Type:     d.TypeArgs[0],
		FuncCall: d.Call,
	}

	d.Pass.RegisterResource(load)
	if id, ok := d.Ident.Get(); ok {
		d.Pass.AddBind(id, load)
	}
}
