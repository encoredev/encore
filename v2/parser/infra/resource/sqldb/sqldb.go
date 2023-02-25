package sqldb

import (
	"go/ast"
	"reflect"

	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type Database struct {
	Name string // The database name
	Doc  string
}

func (d *Database) Kind() resource.Kind { return resource.SQLDatabase }

var DatabaseParser = &resource.Parser{
	Name:      "SQL Database",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/storage/sqldb"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{Name: "Named", PkgPath: "encore.dev/storage/sqldb"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			Parse:       parseNamedSQLDB,
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
		// TODO(andre) this is not really a resource declaration at all
		return resources
	},
}

func parseNamedSQLDB(d parseutil.ParseData) resource.Resource {
	displayName := d.ResourceFunc.NaiveDisplayName()
	if len(d.Call.Args) != 1 {
		d.Pass.Errs.Addf(d.Call.Pos(), "%s expects 1 argument", displayName)
		return nil
	}

	dbName, ok := literals.ParseString(d.Call.Args[0])
	if !ok {
		d.Pass.Errs.Addf(d.Call.Args[0].Pos(), "sqldb.Named requires the first argument to be a string literal, was given a %v.", reflect.TypeOf(d.Call.Args[0]))
		return nil
	}

	if len(dbName) <= 0 {
		d.Pass.Errs.Addf(d.Call.Args[0].Pos(), "sqldb.Named requires the first argument to be a string literal, was given an empty string.")
	}

	return &Database{
		Name: dbName,
		Doc:  d.Doc,
	}
}
