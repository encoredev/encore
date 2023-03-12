package usage

import (
	"go/ast"
	"sort"

	"golang.org/x/exp/slices"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/resource/sqldb"
)

// computeImplicitSQLDBUsage computes the implicit usage of SQLDB resources via package-level
// sqldb.{Query,QueryRow,Exec,etc} calls.
func computeImplicitSQLDBUsage(errs *perr.List, pkgs []*pkginfo.Package, binds []resource.Bind) []Usage {
	// Compute the list of package paths that define SQLDB implicit binds.
	type sqldbBind struct {
		Pkg  paths.Pkg
		Bind resource.Bind
	}

	var sqldbBinds []sqldbBind
	for _, b := range binds {
		if implicit, ok := b.(*resource.ImplicitBind); ok {
			// Is this a SQLDB resource?
			ref := implicit.Resource
			if _, ok := ref.Resource.(*sqldb.Database); ok {
				sqldbBinds = append(sqldbBinds, sqldbBind{Pkg: b.Package().ImportPath, Bind: b})
			} else if ref.Path != nil && ref.Path[0].Kind == resource.SQLDatabase {
				sqldbBinds = append(sqldbBinds, sqldbBind{Pkg: b.Package().ImportPath, Bind: b})
			}
		}
	}

	// Sort them to allow for binary searches.
	slices.SortFunc(sqldbBinds, func(a, b sqldbBind) bool {
		if a.Pkg != b.Pkg {
			return a.Pkg < b.Pkg
		}
		return a.Bind.Pos() < b.Bind.Pos()
	})

	findBind := func(pkg paths.Pkg) (b resource.Bind, ok bool) {
		i := sort.Search(len(sqldbBinds), func(i int) bool {
			return sqldbBinds[i].Pkg >= pkg
		})
		if i >= len(sqldbBinds) {
			return nil, false
		} else if sqldbBinds[i].Pkg != pkg {
			return nil, false
		}
		return sqldbBinds[i].Bind, true
	}

	const sqldbPkg paths.Pkg = "encore.dev/storage/sqldb"

	var usages []Usage
	for _, pkg := range pkgs {
		if !pkg.Imports[sqldbPkg] {
			continue
		}
		for _, file := range pkg.Files {
			if !file.Imports[sqldbPkg] {
				continue
			}

			// The file is using sqldb, so scan its AST for usages.
			insp := file.ASTInspector()
			names := file.Names()
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
					bind, ok := findBind(file.Pkg.ImportPath)
					if !ok {
						errs.Addf(sel.Pos(), "cannot call sqldb.%s outside of a service with a database defined",
							qn.Name)
						return true
					}
					if u := classifySQLDBUsage(file, bind, sel, stack); u != nil {
						usages = append(usages, u)
					}
				}

				return true
			})
		}
	}

	return usages
}

func classifySQLDBUsage(file *pkginfo.File, bind resource.Bind, sel *ast.SelectorExpr, stack []ast.Node) Usage {
	idx := len(stack) - 1
	if idx >= 1 {
		if call, ok := stack[idx-1].(*ast.CallExpr); ok {
			return &MethodCall{
				File:   file,
				Bind:   bind,
				Call:   call,
				Method: sel.Sel.Name,
				Args:   call.Args,
			}
		}
	}

	// Otherwise it's a field access.
	return &FieldAccess{
		File:  file,
		Bind:  bind,
		Expr:  sel,
		Field: sel.Sel.Name,
	}
}
