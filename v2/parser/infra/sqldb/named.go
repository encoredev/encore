package sqldb

import (
	"fmt"
	"go/ast"

	"encr.dev/pkg/errors"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

var NamedParser = &resourceparser.Parser{
	Name: "Named SQL Database",

	InterestingImports: []paths.Pkg{"encore.dev/storage/sqldb"},
	Run: func(p *resourceparser.Pass) {
		name := pkginfo.QualifiedName{Name: "Named", PkgPath: "encore.dev/storage/sqldb"}

		spec := &parseutil.ReferenceSpec{
			Parse:       parseNamedSQLDB,
			MinTypeArgs: 0,
			MaxTypeArgs: 0,
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

func parseNamedSQLDB(d parseutil.ReferenceInfo) {
	if len(d.Call.Args) != 1 {
		d.Pass.Errs.Add(errNamedRequiresDatabaseName(len(d.Call.Args)).AtGoNode(d.Call))
		return
	}

	dbName, ok := literals.ParseString(d.Call.Args[0])
	if !ok {
		d.Pass.Errs.Add(
			errNamedRequiresDatabaseNameString.
				AtGoNode(d.Call.Args[0], errors.AsError(fmt.Sprintf("got %v", parseutil.NodeType(d.Call.Args[0])))),
		)
		return
	}

	if len(dbName) <= 0 {
		d.Pass.Errs.Add(
			errNamedRequiresDatabaseNameString.AtGoNode(d.Call.Args[0], errors.AsError("got an empty string")),
		)
	}

	d.Pass.AddPathBind(d.File, d.Ident, resource.Path{{resource.SQLDatabase, dbName}})
}
