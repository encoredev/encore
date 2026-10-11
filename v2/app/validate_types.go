package app

import (
	"go/ast"

	"encr.dev/pkg/errors"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/internals/schema/schemautil"
	"encr.dev/v2/parser/apis/api/apienc"
)

// validateType validates the type of a field can be marshalled.
// according to Encore's requirements.
//
// This walks the type recursively and validates the whole thing
func (d *Desc) validateType(pc *parsectx.Context, usedAt ast.Node, typ schema.Type) {
	// Convert generic types to their concrete types
	typ = schemautil.ConcretizeGenericType(pc.Errs, typ)

	// Walk the type recursively
	schemautil.Walk(typ, func(t schema.Type) bool {
		switch t := t.(type) {
		case schema.FuncType:
			pc.Errs.Add(
				apienc.ErrFuncNotSupported.
					AtGoNode(t.ASTExpr(), errors.AsError("defined here")).
					AtGoNode(usedAt, errors.AsHelp("used here")),
			)

		case schema.InterfaceType:
			pc.Errs.Add(
				apienc.ErrInterfaceNotSupported.
					AtGoNode(t.ASTExpr(), errors.AsError("defined here")).
					AtGoNode(usedAt, errors.AsHelp("used here")),
			)
		}
		return true
	})
}
