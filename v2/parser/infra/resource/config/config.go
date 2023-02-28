package config

import (
	"go/ast"

	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

// Config represents a config load statement.
type Config struct {
	File *pkginfo.File
}

func (*Config) Kind() resource.Kind         { return resource.Config }
func (c *Config) DeclaredIn() *pkginfo.File { return c.File }

var ConfigParser = &resource.Parser{
	Name:      "Config",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/config"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/config", Name: "Load"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parseLoad,
		}
		var resources []resource.Resource
		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			r := parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
			if r != nil {
				resources = append(resources, r)
			}
		})
		return resources
	},
}

func parseLoad(d parseutil.ParseData) resource.Resource {
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

	return &Config{File: d.File}
}
