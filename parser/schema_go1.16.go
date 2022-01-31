//go:build go1.16 && !go1.18
// +build go1.16,!go1.18

package parser

import (
	"go/ast"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// parseDecl parses the type from a package declaration.
func (p *parser) parseDecl(pkg *est.Package, d *names.PkgDecl, _ typeParameterLookup) *schema.Type {
	key := pkg.ImportPath + "." + d.Name
	decl, ok := p.declMap[key]
	if !ok {
		// We haven't parsed this yet; do so now.
		// Allocate a decl immediately so that we can properly handle
		// recursive types by short-circuiting above the second time we get here.
		id := uint32(len(p.decls))
		typ := d.Spec.(*ast.TypeSpec).Type
		decl = &schema.Decl{
			Id:         id,
			Name:       d.Name,
			Doc:        d.Doc,
			TypeParams: nil,
			Loc:        parseLoc(d.File, typ),
			// Type is set below
		}
		p.declMap[key] = decl
		p.decls = append(p.decls, decl)

		decl.Type = p.resolveType(pkg, d.File, d.Spec.(*ast.TypeSpec).Type, nil)
	}

	return &schema.Type{Typ: &schema.Type_Named{
		Named: &schema.Named{Id: decl.Id},
	}}
}
