package sqldb

import (
	"go/ast"
	"reflect"

	"encr.dev/parser2/infra/internal/literals"
	"encr.dev/parser2/infra/internal/locations"
	"encr.dev/parser2/infra/internal/parseutil"
	"encr.dev/parser2/infra/resources"
	"encr.dev/parser2/internal/pkginfo"
)

type Database struct {
	Name string // The database name
	Doc  string
}

func (d *Database) Kind() resources.Kind { return resources.SQLDatabase }

var DatabaseParser = &resources.Parser{
	Name:      "SQL Database",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/storage/sqldb"},
	Run: func(p *resources.Pass) {
		name := pkginfo.QualifiedName{Name: "Named", PkgPath: "encore.dev/storage/sqldb"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			Parse:       parseNamedSQLDB,
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

func parseNamedSQLDB(d parseutil.ParseData) resources.Resource {
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
