//go:build go1.18
// +build go1.18

package parser

import (
	"go/ast"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/names"
	schema "encr.dev/proto/encore/parser/schema/v1"
)

// parseDecl parses the type from a package declaration.
func (p *parser) parseDecl(pkg *est.Package, d *names.PkgDecl, typeParameters typeParameterLookup) *schema.Type {
	key := pkg.ImportPath + "." + d.Name
	decl, ok := p.declMap[key]
	if !ok {
		// We haven't parsed this yet; do so now.
		// Allocate a decl immediately so that we can properly handle
		// recursive types by short-circuiting above the second time we get here.
		id := uint32(len(p.decls))
		spec, ok := d.Spec.(*ast.TypeSpec)
		if !ok {
			p.errf(d.Spec.Pos(), "unable to get TypeSpec from PkgDecl spec")
			p.errors.Abort()
		}

		decl = &schema.Decl{
			Id:         id,
			Name:       d.Name,
			Doc:        d.Doc,
			TypeParams: nil,
			Loc:        parseLoc(d.File, spec.Type),
			// Type is set below
		}
		p.declMap[key] = decl
		p.decls = append(p.decls, decl)

		typeParameterLookup := make(typeParameterLookup)

		// If this is a parameterized declaration, get the type parameters
		if spec.TypeParams != nil {
			decl.TypeParams = make([]*schema.TypeParameter, len(spec.TypeParams.List))

			for idx, typeParameter := range spec.TypeParams.List {
				if len(typeParameter.Names) != 1 {
					p.errf(typeParameter.Pos(), "type parameter had more than 1 name")
					p.errors.Abort()
				}

				name := typeParameter.Names[0].Name

				decl.TypeParams[idx] = &schema.TypeParameter{
					Name: name,
				}

				typeParameterLookup[name] = &schema.TypeParameterRef{
					DeclId:   id,
					ParamIdx: uint32(idx),
				}
			}
		}

		decl.Type = p.resolveType(pkg, d.File, spec.Type, typeParameterLookup)
	}

	return &schema.Type{Typ: &schema.Type_Named{
		Named: &schema.Named{Id: decl.Id},
	}}
}
