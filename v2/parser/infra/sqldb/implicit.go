package sqldb

import (
	"go/ast"
	"sort"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/parser/resource"
	"encr.dev/v2/parser/resource/usage"
)

// ComputeImplicitUsage computes the implicit usage of SQLDB resources via package-level
// sqldb.{Query,QueryRow,Exec,etc} calls.
func ComputeImplicitUsage(errs *perr.List, pkgs []*pkginfo.Package, binds []resource.Bind) []usage.Expr {
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
			if _, ok := ref.Resource.(*Database); ok {
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
		if i < len(sqldbBinds) && sqldbBinds[i].Pkg == pkg {
			return sqldbBinds[i].Bind, true
		} else if i > 0 && sqldbBinds[i-1].Pkg.LexicallyContains(pkg) {
			return sqldbBinds[i-1].Bind, true
		}
		return nil, false
	}

	const sqldbPkg paths.Pkg = "encore.dev/storage/sqldb"

	var usages []usage.Expr
	for _, pkg := range pkgs {
		if _, found := pkg.Imports[sqldbPkg]; !found {
			continue
		}
		for _, file := range pkg.Files {
			if _, found := file.Imports[sqldbPkg]; !found {
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
					validUsage := false
					if bind, ok := findBind(file.Pkg.ImportPath); ok {
						if u := classifySQLDBUsage(file, bind, sel, stack); u != nil {
							usages = append(usages, u)
							validUsage = true
						}
					}
					if !validUsage {
						errs.Add(errInvalidPkgLevelQuery(qn.Name).AtGoNode(sel))
					}
				}

				return true
			})
		}
	}

	return usages
}

func classifySQLDBUsage(file *pkginfo.File, bind resource.Bind, sel *ast.SelectorExpr, stack []ast.Node) usage.Expr {
	idx := len(stack) - 1
	if idx >= 1 {
		if call, ok := stack[idx-1].(*ast.CallExpr); ok {
			return &usage.MethodCall{
				File:   file,
				Bind:   bind,
				Call:   call,
				Method: sel.Sel.Name,
				Args:   call.Args,
			}
		}
	}

	// Otherwise it's a field access.
	return &usage.FieldAccess{
		File:  file,
		Bind:  bind,
		Expr:  sel,
		Field: sel.Sel.Name,
	}
}
