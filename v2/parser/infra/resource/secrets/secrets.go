package secrets

import (
	"go/ast"
	"go/token"

	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/resource"
)

// Secrets represents a secrets struct.
type Secrets struct {
	Keys []string // Secret keys to load
}

func (*Secrets) Kind() resource.Kind { return resource.Secrets }

var SecretsParser = &resource.Parser{
	Name:      "Secrets",
	DependsOn: nil,

	RequiredImports: nil, // applies to all packages
	Run: func(p *resource.Pass) []resource.Resource {
		secrets := p.Pkg.Names().PkgDecls["secrets"]
		if secrets == nil || secrets.Type != token.VAR {
			return nil // nothing to do
		}

		// Note: we can't use schema.ParseTypeDecl since this is not a type declaration.
		// Resolve the type expression manually instead.
		spec := secrets.Spec.(*ast.ValueSpec)
		if spec.Type == nil {
			p.Errs.Add(spec.Pos(), "secrets variable must be a struct")
			return nil
		} else if len(spec.Names) != 1 {
			p.Errs.Add(spec.Pos(), "secrets variable must be declared separately")
			return nil
		} else if len(spec.Values) != 0 {
			p.Errs.Add(spec.Pos(), "secrets variable must not be given a value")
			return nil
		}

		st, ok := p.SchemaParser.ParseType(secrets.File, spec.Type).(schema.StructType)
		if !ok {
			p.Errs.Add(spec.Pos(), "secrets variable must be a struct")
			return nil
		}

		res := &Secrets{}
		for _, f := range st.Fields {
			if f.IsAnonymous() {
				p.Errs.Add(f.AST.Pos(), "secrets: anonymous fields are not allowed")
				continue
			}
			if !schemautil.IsBuiltinKind(f.Type, schema.String) {
				p.Errs.Addf(f.AST.Pos(), "secrets: field %s is not of type string", f.Name.MustGet())
				continue
			}
			res.Keys = append(res.Keys, f.Name.MustGet())
		}

		return []resource.Resource{res}
	},
}
