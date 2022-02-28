package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"golang.org/x/tools/go/ast/astutil"

	"encr.dev/compiler/internal/rewrite"
	"encr.dev/parser/est"
)

// rewritePkg writes out modified files to targetDir.
func (b *builder) rewritePkg(pkg *est.Package, targetDir string) error {
	fset := b.res.FileSet
	seenWrappers := make(map[string]bool)
	var wrappers []*est.RPC
	for _, file := range pkg.Files {
		if len(file.References) == 0 {
			// No references to other RPCs, we can skip it immediately
			continue
		}

		rewrittenPkgs := make(map[*est.Package]bool)
		rw := rewrite.New(file.Contents, file.Token.Base())

		useExceptions := make(map[*ast.SelectorExpr]bool)
		astutil.Apply(file.AST, func(c *astutil.Cursor) bool {
			node := c.Node()
			rewrite, ok := file.References[node]
			if !ok {
				return true
			}

			switch rewrite.Type {
			case est.SQLDBNode:
				// Do nothing
				return true

			case est.RLogNode:
				// do nothing
				return true

			case est.RPCRefNode:
				rpc := rewrite.RPC
				wrapperName := "__encore_" + rpc.Svc.Name + "_" + rpc.Name
				node := c.Node()

				// Capture rewrites that should be ignored when computing if an import
				// is still in use. The func is generally a SelectorExpr but if we call
				// an API within the same package it's an ident, and can be safely ignored.
				if sel, ok := node.(*ast.SelectorExpr); ok {
					useExceptions[sel] = true
				}

				rw.Replace(node.Pos(), node.End(), []byte(wrapperName))
				rewrittenPkgs[rpc.Svc.Root] = true

				if !seenWrappers[wrapperName] {
					wrappers = append(wrappers, rpc)
					seenWrappers[wrapperName] = true
				}
				return true

			case est.RPCDefNode:
				// Do nothing
				return true

			case est.SecretsNode:
				spec := c.Node().(*ast.ValueSpec)

				var buf bytes.Buffer
				buf.WriteString("{\n")
				for _, secret := range pkg.Secrets {
					fmt.Fprintf(&buf, "\t%s: __encore_runtime.LoadSecret(%s),\n", secret, strconv.Quote(secret))
				}
				ep := fset.Position(spec.End())
				fmt.Fprintf(&buf, "}/*line :%d:%d*/", ep.Line, ep.Column)

				rw.Insert(spec.Type.Pos(), []byte("= "))
				rw.Insert(spec.End(), buf.Bytes())

				decl := file.AST.Decls[0]
				ln := fset.Position(decl.Pos())
				rw.Insert(decl.Pos(), []byte(fmt.Sprintf("import __encore_runtime %s\n/*line :%d:%d*/", strconv.Quote("encore.dev/runtime"), ln.Line, ln.Column)))
				return true

			default:
				panic(fmt.Sprintf("unhandled rewrite type: %v", rewrite.Type))
			}
		}, nil)

		// Determine if we have some imports that are now unused that we should remove.
		for pkg := range rewrittenPkgs {
			if !usesImport(file.AST, pkg.Name, pkg.ImportPath, useExceptions) {
				spec, decl, ok := findImport(file.AST, pkg.ImportPath)
				if ok {
					// If the decl contains multiple imports, only delete the spec
					if len(decl.Specs) > 1 {
						rw.Delete(spec.Pos(), spec.End())
					} else {
						rw.Delete(decl.Pos(), decl.End())
					}
				}
			}
		}

		// Write out the file
		name := filepath.Base(file.Path)
		dst := filepath.Join(targetDir, name)
		if err := ioutil.WriteFile(dst, rw.Data(), 0644); err != nil {
			return err
		}
		b.addOverlay(file.Path, dst)
	}

	if len(wrappers) > 0 {
		name := "encore_internal__rpc_wrappers.go"
		wrapperPath := filepath.Join(targetDir, name)
		if err := b.generateWrappers(pkg, wrappers, wrapperPath); err != nil {
			return err
		}
		b.addOverlay(filepath.Join(pkg.Dir, name), wrapperPath)
	}
	return nil
}
