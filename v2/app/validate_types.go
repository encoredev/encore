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
func (d *Desc) validateType(pc *parsectx.Context, usedAt ast.Node, typ schema.Type, isAuthType bool) {
	// Convert generic types to their concrete types
	typ = schemautil.ConcretizeGenericType(pc.Errs, typ)

	// Walk the type recursively
	schemautil.Walk(typ, func(t schema.Type) bool {
		switch t := t.(type) {
		case schema.StructType:
			for _, field := range t.Fields {
				if field.IsAnonymous() {
					// We don't support anonymous fields anywhere within
					// Encore types that we need to marshal.
					pc.Errs.Add(
						apienc.ErrAnonymousFieldsNotSupported.
							AtGoNode(field.AST, errors.AsError("defined here")).
							AtGoNode(usedAt, errors.AsHelp("used here")),
					)
				} else if isAuthType && !field.IsExported() {
					// We don't allow unexported fields in auth types
					// because we need to ensure that the auth type if fully
					// serializable between services - otherwise we risk a service
					// misinterpreting an unexported zero value as a valid value.
					// (i.e "user.banned" is true, but doesn't get marshalled and so becomes false)
					pc.Errs.Add(
						apienc.ErrUnexportedFieldsNotSupported.
							AtGoNode(field.AST.Names[0], errors.AsError("defined here")).
							AtGoNode(usedAt, errors.AsHelp("used here")),
					)
				}
			}

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
