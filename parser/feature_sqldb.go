package parser

import (
	"go/ast"
	"reflect"
	"strings"

	"encr.dev/internal/experiment"
	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
)

func init() {
	registerResource(est.SQLDBResource, "shared database", "https://encore.dev/docs/how-to/share-db-between-services", "sqldb", sqldbImportPath, experiment.None)

	registerResourceCreationParser(est.SQLDBResource, "Named", 0, (*parser).parseNamedSQLDB, experiment.None, locations.AllowedIn(locations.Variable).ButNotIn(locations.Function))
}

func (p *parser) parseNamedSQLDB(file *est.File, _ *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) != 1 {
		p.errf(callExpr.Pos(), "sqldb.Named requires exactly one argument, the database name given as a string literal. For example `sqldb.Named(\"my-svc\")`")
		return nil
	}

	dbName, ok := litString(callExpr.Args[0])
	if !ok {
		p.errf(callExpr.Args[0].Pos(), "sqldb.Named requires the first argument to be a string literal, was given a %v.", reflect.TypeOf(callExpr.Args[0]))
		return nil
	}
	dbName = strings.TrimSpace(dbName)
	if len(dbName) <= 0 {
		p.errf(callExpr.Args[0].Pos(), "sqldb.Named requires the first argument to be a string literal, was given an empty string.")
		return nil
	}

	return &est.SQLDB{
		DeclFile: file,
		DeclName: ident,
		DBName:   dbName,
	}
}
