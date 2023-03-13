//go:build ignore

package sqldb

import (
	"go/ast"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/resourceparser"
)

const sqldbPkg paths.Pkg = "encore.dev/storage/sqldb"

var ImplicitBindParser = &resourceparser.Parser{
	Name: "Implicit SQLDB binds",

	InterestingImports: []paths.Pkg{sqldbPkg},
	Run: func(p *resourceparser.Pass) {
		defer func() {
			if r := recover(); r != nil {
				if _, isBailout := r.(bailout); isBailout {
					p.AddPathBind(ast.NewIdent("_"), resource.Path{{resource.SQLDatabase}})
				}
			}
		}()
		for _, f := range p.Pkg.Files {
			if f.Imports[sqldbPkg] {
				insp := f.ASTInspector()
				names := f.Names()
				insp.WithStack([]ast.Node{(*ast.SelectorExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
					if !push {
						return true
					}

					sel := n.(*ast.SelectorExpr)
					qn, ok := names.ResolvePkgLevelRef(sel)
					if !ok || qn.PkgPath != sqldbPkg {
						return true
					}

					switch qn.Name {
					case "Exec", "ExecTx", "QueryRow", "QueryRowTx", "Query", "QueryTx", "Begin":

					}
				})
			}
		}
		p.RegisterResource(res)
	},
}

type bailout struct{}
