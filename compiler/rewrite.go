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
				wrapperName := "EncoreInternal_Call" + rpc.Name
				node := c.Node()

				if sel, ok := node.(*ast.SelectorExpr); ok {
					rw.Replace(sel.Sel.Pos(), sel.Sel.End(), []byte(wrapperName))
				} else {
					rw.Replace(node.Pos(), node.End(), []byte(wrapperName))
				}
				rewrittenPkgs[rpc.Svc.Root] = true
				return true

			case est.RPCDefNode:
				// Do nothing
				return true

			case est.SecretsNode:
				spec := c.Node().(*ast.ValueSpec)

				var buf bytes.Buffer
				buf.WriteString("{\n")
				for _, secret := range pkg.Secrets {
					fmt.Fprintf(&buf, "\t%s: __encore_app.LoadSecret(%s),\n", secret, strconv.Quote(secret))
				}
				ep := fset.Position(spec.End())
				fmt.Fprintf(&buf, "}/*line :%d:%d*/", ep.Line, ep.Column)

				rw.Insert(spec.Type.Pos(), []byte("= "))
				rw.Insert(spec.End(), buf.Bytes())

				decl := file.AST.Decls[0]
				ln := fset.Position(decl.Pos())
				rw.Insert(decl.Pos(), []byte(fmt.Sprintf("import __encore_app %s\n/*line :%d:%d*/", strconv.Quote("encore.dev/appruntime/app/appinit"), ln.Line, ln.Column)))
				return true

			case est.CronJobNode:
				return true

			case est.PubSubTopicDefNode, est.PubSubPublisherNode, est.PubSubSubscriberNode:
				return true

			case est.CacheClusterDefNode:
				return true

			case est.CacheKeyspaceDefNode:
				keyspace := rewrite.Res.(*est.CacheKeyspace)
				cfgLit := keyspace.ConfigLit

				insertPos := cfgLit.Lbrace + 1
				ep := fset.Position(insertPos)
				defLoc := b.res.Nodes[keyspace.Svc.Root][node].Id

				rw.Insert(insertPos, []byte(fmt.Sprintf(
					"EncoreInternal_DefLoc: %d, EncoreInternal_KeyMapper: %s,/*line :%d:%d*/",
					defLoc, b.codegen.CacheKeyspaceKeyMapperName(keyspace),
					ep.Line, ep.Column,
				)))
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

	return nil
}
