package config

import (
	"go/ast"

	"encr.dev/parser2/infra/internal/locations"
	"encr.dev/parser2/infra/internal/parseutil"
	"encr.dev/parser2/infra/resources"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema/schemautil"
)

// Config represents a config load statement.
type Config struct {
}

func (*Config) Kind() resources.Kind { return resources.Config }

var ConfigParser = &resources.Parser{
	Name:      "Config",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/config"},
	Run: func(p *resources.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/config", Name: "Load"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parseLoad,
		}
		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parseLoad(d parseutil.ParseData) resources.Resource {
	errs := d.Pass.Errs

	if len(d.Call.Args) > 0 {
		errs.Add(d.Call.Pos(), "config.Load expects no arguments")
		return nil
	}

	// Resolve the named struct used for the config type
	ref, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		errs.Add(d.TypeArgs[0].ASTExpr().Pos(), "config.Load expects a named struct type as its type argument")
		return nil
	}
	_ = ref

	return &Config{}
}
