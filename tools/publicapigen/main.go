package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

var (
	fset      = token.NewFileSet()
	files     = map[string]*ast.File{}
	constants = map[string]map[string]ast.BasicLit{}
	types     = map[string]map[string]ast.Expr{}
)

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Caller().Logger()
	log.Info().Msg("Generating Public API")

	// Walk the directory tree and parse all the Go files
	log.Info().Msg("Parsing source files...")
	if err := walkDir(filepath.Join(repoDir(), "runtime"), "./", readAST); err != nil {
		log.Fatal().Err(err).Msg("Unable to walk runtime directory to parse Go files")
	}

	// Register all consts and types in our private files, just in case we reference them in the public API
	log.Info().Msg("Registering types...")
	for fileName, fAST := range files {
		if isPrivateFile(fileName) {
			log.Debug().Str("file", fileName).Msg("Registering types and constants from private implementation files")
			registerTypes(fileName, fAST)
		}
	}

	// Then rewrite all the AST to remove implementations
	log.Info().Msg("Rewriting AST to remove implementations and unexported items...")
	for fileName, fAST := range files {

		log.Debug().Str("file", fileName).Msg("Rewriting AST")
		if err := rewriteAST(fAST); err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("Unable to rewrite AST")
		}

		if len(fAST.Decls) == 0 && fAST.Doc == nil {
			log.Debug().Str("file", fileName).Msg("Removing file as there are no exported decelerations or package comments")
			delete(files, fileName)
		}
	}

	// Then write the AST to a file
	outDir := outDir()
	log.Info().Str("out", outDir).Msg("Writing public API files...")
	for fileName, fAST := range files {
		if isPrivateFile(fileName) {
			// "runtime" is a private package as are internal packages
			// any files suffixed with _internal.go are also private and are considered unstable API's
			continue
		}

		log.Debug().Str("file", fileName).Msg("Writing public API file")

		// Print the AST to a buffer
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, fAST); err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("Unable to write AST to file")
		}

		// Then pass that to goimports to format the imports
		imports.LocalPrefix = "encore.dev"
		formatted, err := imports.Process(fileName, buf.Bytes(), nil)
		if err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("Unable to process imports")
		}

		// Now let's write the file out
		outputFile := filepath.Join(outDir, fileName)
		outputDir := filepath.Dir(outputFile)

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatal().Err(err).Str("dir", outputDir).Msg("Unable to create output directory")
		}

		if err := os.WriteFile(outputFile, formatted, 0644); err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("Unable to write file")
		}
	}

	log.Info().Msg("Done")
}

func registerTypes(name string, fAST *ast.File) {
	pkg := filepath.Base(filepath.Dir(name))
	if _, found := types[pkg]; !found {
		types[pkg] = map[string]ast.Expr{}
		constants[pkg] = map[string]ast.BasicLit{}
	}

	for _, decl := range fAST.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name != nil && s.Name.IsExported() {
						log.Debug().Str("type", s.Name.Name).Msg("Registering type")
						types[pkg][s.Name.Name] = s.Type
					}
				case *ast.ValueSpec:
					if d.Tok == token.CONST {
						for i, name := range s.Names {
							if len(s.Values) <= i {
								break
							}
							if basic, ok := s.Values[i].(*ast.BasicLit); ok && name.IsExported() {
								log.Debug().Str("const", name.Name).Msg("Registering basic const")
								constants[pkg][name.Name] = *basic
							}
						}
					}
				}
			}
		}
	}
}

// readAST parses the AST of all non-test Go files in a directory and stores it in the files map
func readAST(path, rel string, file []os.FileInfo) error {
	for _, f := range file {
		if !strings.HasSuffix(f.Name(), ".go") {
			// ignore non-go files
			continue
		}

		if strings.HasSuffix(f.Name(), "_test.go") {
			// Ignore test files
			continue
		}

		log.Debug().Str("rel", rel).Str("file", f.Name()).Msg("Parsing file")

		fAST, err := parser.ParseFile(fset, filepath.Join(path, f.Name()), nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("error parsing %s: %w", f.Name(), err)
		}

		// We only want to track comments if they are part of a decl we're keeping
		// so we need to nil out this field on the file so they aren't tracked globally
		fAST.Comments = nil
		files[filepath.Join(rel, f.Name())] = fAST
	}

	return nil
}

func rewriteAST(f *ast.File) error {
	astutil.Apply(
		f,
		func(c *astutil.Cursor) bool {
			switch node := c.Node().(type) {
			case *ast.FuncDecl:
				if mustKeep(node.Doc) {
					return false
				}

				// Should we delete this function declaration if it's unexported or a receiver on an unexported object?
				shouldDelete := !node.Name.IsExported()
				if node.Recv != nil {
					for i, field := range node.Recv.List {
						if ident := typeName(field.Type); ident != nil && !ident.IsExported() {
							shouldDelete = true
							break
						}

						// Remove the node name from the receiver as we won't reference it inside an empty body
						// if we keep the field
						node.Recv.List[i].Names = nil
					}
				}

				if shouldDelete {
					clearCommentGroup(node.Doc)
					c.Delete()
					return false
				}

				start := token.NoPos
				end := token.NoPos

				if node.Body != nil {
					start = node.Body.Lbrace
					end = node.Body.Rbrace
				}

				// If we are keeping the function, replace the implementation with a panic
				// as the code we're generating is only to help the IDE and users understand our API
				// but isn't intended to be used in running apps
				node.Body = &ast.BlockStmt{
					Lbrace: token.NoPos,
					List: []ast.Stmt{
						&ast.ExprStmt{
							X: &ast.CallExpr{
								Fun:    ast.NewIdent("panic"),
								Lparen: start,
								Args: []ast.Expr{
									&ast.BasicLit{
										ValuePos: token.NoPos,
										Kind:     token.STRING,
										Value:    "\"encore apps must be run using the encore command\"",
									},
								},
								Ellipsis: 0,
								Rparen:   end,
							},
						},
					},
					Rbrace: token.NoPos,
				}
				return false

			case *ast.TypeSpec:
				keep := node.Name.IsExported()
				if mustKeep(node.Doc, node.Comment) {
					keep = true
				}

				// Remove unexported types
				if !keep {
					c.Delete()
					return false
				} else if sel, ok := node.Type.(*ast.SelectorExpr); ok {
					// else if we have a selector, rewrite the package name to see if this is an export from
					// one of our private packages, in which case we want to copy it into the public API
					if pkg, ok := sel.X.(*ast.Ident); ok && types[pkg.Name] != nil {
						typ, found := types[pkg.Name][sel.Sel.Name]
						if found {
							node.Assign = token.NoPos // remove an alias assignment (as it was an alias for our own types)
							node.Type = typ           // replace the type with the actual type
						}
					}
				}

			case *ast.ValueSpec:
				// Remove unexported variables and constants
				keep := false
				for i, name := range node.Names {
					if name.IsExported() {
						keep = true
					} else {
						node.Names[i] = ast.NewIdent("_")
					}
				}

				if mustKeep(node.Doc, node.Comment) {
					keep = true
				}

				if !keep {
					c.Delete()
					return false
				}

			case *ast.SelectorExpr:
				// else if we have a selector, rewrite the package name to see if this is an export from
				// one of our private packages, in which case we want to copy it into the public API
				if pkg, ok := node.X.(*ast.Ident); ok && constants[pkg.Name] != nil {
					typ, found := constants[pkg.Name][node.Sel.Name]
					if found {
						typ.ValuePos = token.NoPos
						c.Replace(&typ)
					}
				}
			case *ast.GenDecl:
				if mustKeep(node.Doc) {
					return false
				}

			case *ast.Field:
				// Keep exported fields if there's a doc saying we should
				keep := mustKeep(node.Doc, node.Comment)

				if !keep {
					// Remove unexported fields from structs
					for i, name := range node.Names {
						if name.IsExported() {
							keep = true
						} else {
							node.Names[i] = ast.NewIdent("_")
						}
					}
				}

				if !keep {
					c.Delete()
				}
				return false
			}

			return true
		},
		func(c *astutil.Cursor) bool {
			switch node := c.Node().(type) {
			case *ast.GenDecl:
				if len(node.Specs) == 0 {
					clearCommentGroup(node.Doc)
					c.Delete()
				}

			case *ast.StructType:
				if len(node.Fields.List) == 0 {
					node.Fields.List = append(node.Fields.List, &ast.Field{
						Doc:   nil,
						Names: []*ast.Ident{ast.NewIdent("_")},
						Type:  ast.NewIdent("int"),
						Tag:   nil,
						Comment: &ast.CommentGroup{List: []*ast.Comment{
							{
								Slash: token.NoPos,
								Text:  "// for godoc to show unexported fields",
							},
						}},
					})
				}
			}

			return true
		},
	)
	return nil
}

// walkDir recursively descends path, calling walkFn for directory
func walkDir(dir, rel string, f func(path, rel string, files []os.FileInfo) error) error {
	if rel == "types/uuid" {
		// we don't want to rewrite this package
		return nil
	}

	log.Debug().Str("rel", rel).Msg("Walking directory")
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	// Split the files and dirs
	var dirs, files []os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry)
		} else {
			files = append(files, entry)
		}
	}

	if err := f(dir, rel, files); err != nil {
		return fmt.Errorf("error walking %s: %w", rel, err)
	}

	for _, d := range dirs {
		dir2 := filepath.Join(dir, d.Name())
		rel2 := path.Join(rel, d.Name())

		if err := walkDir(dir2, rel2, f); err != nil {
			return err
		}
	}
	return nil
}

func repoDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("Unable to get caller location")
	}

	publicapigenDir := filepath.Dir(file)
	toolsDir := filepath.Dir(publicapigenDir)
	encoreDir := filepath.Dir(toolsDir)

	return encoreDir
}

func outDir() string {
	srcDir := filepath.Dir(repoDir())
	return filepath.Join(srcDir, "encore.dev")
}

// typeName returns the identifier of the expression unwrapping any pointers, selectors or generics, or nil if it is not an identifier
func typeName(node ast.Expr) *ast.Ident {
	// Remove any wrapped references
	for {
		if star, ok := node.(*ast.StarExpr); ok {
			node = star.X
			continue
		}

		if index, ok := node.(*ast.IndexExpr); ok {
			node = index.X
			continue
		}

		if indexList, ok := node.(*ast.IndexListExpr); ok {
			node = indexList.X
			continue
		}

		if selector, ok := node.(*ast.SelectorExpr); ok {
			node = selector.X
		}

		break
	}

	if ident, ok := node.(*ast.Ident); ok {
		return ident
	}
	return nil
}

func clearCommentGroup(node *ast.CommentGroup) {
	if node == nil {
		return
	}
	node.List = []*ast.Comment{node.List[0]}
	node.List[0].Text = "  " // double space to prevent a panic when printing the file back out
}

func mustKeep(nodes ...*ast.CommentGroup) bool {
	for _, node := range nodes {
		if node != nil && node.List != nil {
			for i, comment := range node.List {
				if comment.Text == "//encore:keep" {
					if i == 0 {
						comment.Text = "  "
					} else {
						comment.Text = "//" // empty comment line as I want the docs to remain active, but I can't remove this without causing a blank line between the comment group and what ever it's associated with
					}
					return true
				}
			}
		}
	}
	return false
}

func isPrivateFile(fileName string) bool {
	return strings.HasPrefix(fileName, "runtime/") ||
		strings.Contains(fileName, "internal/") ||
		strings.HasSuffix(fileName, "_internal.go")
}
