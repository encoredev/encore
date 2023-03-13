package secrets

import (
	"fmt"
	"go/ast"
	"go/token"

	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	"encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/resource"
)

// Secrets represents a secrets struct.
type Secrets struct {
	AST   *ast.StructType
	File  *pkginfo.File // Where the secrets struct is declared
	Ident *ast.Ident    // The identifier of the secrets struct
	Keys  []string      // Secret keys to load

	// Spec is the value spec that defines the 'secrets' variable.
	Spec *ast.ValueSpec
}

type SecretKey struct {
	Name string
}

func (*Secrets) Kind() resource.Kind         { return resource.Secrets }
func (s *Secrets) Package() *pkginfo.Package { return s.File.Pkg }
func (s *Secrets) ASTExpr() ast.Expr         { return s.AST }

var SecretsParser = &resource.Parser{
	Name:               "Secrets",
	InterestingImports: resource.RunAlways,

	Run: func(p *resource.Pass) {
		secrets := p.Pkg.Names().PkgDecls["secrets"]
		if secrets == nil || secrets.Type != token.VAR {
			return // nothing to do
		}

		// Note: we can't use schema.ParseTypeDecl since this is not a type declaration.
		// Resolve the type expression manually instead.
		spec := secrets.Spec.(*ast.ValueSpec)
		if spec.Type == nil {
			p.Errs.Add(errSecretsMustBeStruct.AtGoNode(spec, errors.AsError(fmt.Sprintf("got %s", parseutil.NodeType(spec)))))
			return
		} else if len(spec.Names) != 1 {
			p.Errs.Add(errSecretsDefinedSeperately.AtGoNode(spec))
			return
		} else if len(spec.Values) != 0 {
			p.Errs.Add(errSecretsGivenValue.AtGoNode(spec.Values[0]))
			return
		}

		st, ok := p.SchemaParser.ParseType(secrets.File, spec.Type).(schema.StructType)
		if !ok {
			p.Errs.Add(errSecretsMustBeStruct.AtGoNode(spec, errors.AsError(fmt.Sprintf("got %s", parseutil.NodeType(spec)))))
			return
		}

		res := &Secrets{
			AST:   spec.Type.(*ast.StructType),
			File:  secrets.File,
			Spec:  spec,
			Ident: spec.Names[0],
		}

		for _, f := range st.Fields {
			if f.IsAnonymous() {
				p.Errs.Add(errAnonymousFields.AtGoNode(f.AST))
				continue
			}
			if !schemautil.IsBuiltinKind(f.Type, schema.String) {
				p.Errs.Add(errSecretsMustBeString.AtGoNode(f.AST.Type, errors.AsError(fmt.Sprintf("got %s", literals.PrettyPrint(f.Type.ASTExpr())))))
				continue
			}
			res.Keys = append(res.Keys, f.Name.MustGet())
		}

		p.RegisterResource(res)
		p.AddBind(res.Ident, res)
	},
}
