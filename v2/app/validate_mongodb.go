package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/infra/mongodb"
)

func (d *Desc) validateMongoDB(pc *parsectx.Context, result *parser.Result) {
	foundDBs := make(map[string]*mongodb.Database)

	dbs := parser.Resources[*mongodb.Database](result)
	for _, db := range dbs {
		if previous, ok := foundDBs[db.Name]; ok {
			pc.Errs.Add(
				mongodb.ErrDuplicateNames.
					AtGoNode(db.AST.Args[0]).
					AtGoNode(previous.AST.Args[0]),
			)
		}
		foundDBs[db.Name] = db
	}

	// Check for usages outside of services
	for _, db := range dbs {
		for _, u := range d.ResourceUsageOutsideServices[db] {
			if !u.DeclaredIn().TestFile {
				pc.Errs.Add(
					errResourceUsedOutsideService.AtGoNode(u, errors.AsError("used here")),
				)
			}
		}
	}
}
