package mongodb

import (
	"go/ast"
	"go/token"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	literals "encr.dev/v2/parser/infra/internal/literals"
	parseutil "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

// Database represents a MongoDB database declared with mongodb.NewDatabase.
type Database struct {
	AST  *ast.CallExpr
	File *pkginfo.File
	Name string // The unique name of the database
	Doc  string // The documentation on the database
}

func (d *Database) Kind() resource.Kind       { return resource.MongoDatabase }
func (d *Database) Package() *pkginfo.Package { return d.File.Pkg }
func (d *Database) ASTExpr() ast.Expr         { return d.AST }
func (d *Database) ResourceName() string      { return d.Name }
func (d *Database) Pos() token.Pos            { return d.AST.Pos() }
func (d *Database) End() token.Pos            { return d.AST.End() }
func (d *Database) SortKey() string           { return d.Name }

var DatabaseParser = &resourceparser.Parser{
	Name: "MongoDB Database",

	InterestingImports: []paths.Pkg{"encore.dev/storage/mongodb"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{PkgPath: "encore.dev/storage/mongodb", Name: "NewDatabase"}

		spec := &parseutil.ReferenceSpec{
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
			Parse:       parseDatabase,
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

func parseDatabase(d parseutil.ReferenceInfo) {
	errs := d.Pass.Errs

	if len(d.Call.Args) != 2 {
		errs.Add(errNewDatabaseArgCount(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	databaseName := parseutil.ParseResourceName(d.Pass.Errs, "mongodb.NewDatabase", "database name",
		d.Call.Args[0], parseutil.SnakeName, "")
	if databaseName == "" {
		// we already reported the error inside ParseResourceName
		return
	}

	// The config must be a struct literal. DatabaseConfig has no fields yet,
	// so there is nothing to decode.
	if _, ok := literals.ParseStruct(d.Pass.Errs, d.File, "mongodb.DatabaseConfig", d.Call.Args[1]); !ok {
		return // error reported by ParseStruct
	}

	db := &Database{
		AST:  d.Call,
		File: d.File,
		Name: databaseName,
		Doc:  d.Doc,
	}
	d.Pass.RegisterResource(db)
	d.Pass.AddBind(d.File, d.Ident, db)
}
