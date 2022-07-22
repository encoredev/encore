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
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

type registeredType struct {
	node ast.Expr
	docs *ast.CommentGroup
}

type registeredConstant struct {
	node ast.Expr
	typ  ast.Expr
	docs *ast.CommentGroup
}

var (
	fset        = token.NewFileSet()
	files       = map[string]*ast.File{}
	constants   = map[string]map[string]registeredConstant{}
	types       = map[string]map[string]registeredType{}
	typesToDrop = map[string]map[string]bool{}
)

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Caller().Logger()
	log.Info().Msg("generating public api")

	// Walk the directory tree and parse all the Go files
	log.Info().Msg("parsing source files...")
	if err := walkDir(filepath.Join(repoDir(), "runtime"), "./", readAST); err != nil {
		log.Fatal().Err(err).Msg("unable to walk runtime directory to parse go files")
	}

	// Register all consts and types in our private files, just in case we reference them in the public API
	log.Info().Msg("registering types...")
	for fileName, fAST := range files {
		if isPrivateFile(fileName) {
			log.Debug().Str("file", fileName).Msg("registering types and constants from private implementation files")
			registerTypes(fileName, fAST)
		}
		registerTypesToDrop(fAST)
	}

	// Then rewrite all the AST to remove implementations
	log.Info().Msg("rewriting ast to remove implementations and unexported items...")
	for fileName, fAST := range files {
		log.Debug().Str("file", fileName).Msg("rewriting ast")
		if err := rewriteAST(fAST); err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("unable to rewrite ast")
		}

		if len(fAST.Decls) == 0 && fAST.Doc == nil {
			log.Debug().Str("file", fileName).Msg("removing file as there are no exported decelerations or package comments")
			delete(files, fileName)
		}
	}

	// Then write the AST to a file
	outDir := outDir()
	log.Info().Str("out", outDir).Msg("writing public api files...")
	for fileName, fAST := range files {
		if isPrivateFile(fileName) {
			// "runtime" is a private package as are internal packages
			// any files suffixed with _internal.go are also private and are considered unstable API's
			continue
		}

		log.Debug().Str("file", fileName).Msg("writing public api file")

		// Print the AST to a buffer
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, fAST); err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("unable to write ast to file")
		}

		// Then pass that to goimports to format the imports
		imports.LocalPrefix = "encore.dev"
		formatted, err := imports.Process(fileName, buf.Bytes(), nil)
		if err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("unable to process imports")
		}

		// Now let's write the file out
		outputFile := filepath.Join(outDir, fileName)
		outputDir := filepath.Dir(outputFile)

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatal().Err(err).Str("dir", outputDir).Msg("unable to create output directory")
		}

		if err := os.WriteFile(outputFile, formatted, 0644); err != nil {
			log.Fatal().Err(err).Str("file", fileName).Msg("unable to write file")
		}
	}

	log.Info().Msg("done")
}

func registerTypes(name string, fAST *ast.File) {
	pkg := filepath.Base(filepath.Dir(name))
	if _, found := types[pkg]; !found {
		types[pkg] = map[string]registeredType{}
		constants[pkg] = map[string]registeredConstant{}
	}

	for _, decl := range fAST.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name != nil && s.Name.IsExported() {
						log.Debug().Str("type", s.Name.Name).Msg("registering type")
						types[pkg][s.Name.Name] = registeredType{s.Type, removePosFromCommentGroup(d.Doc)}
					}
				case *ast.ValueSpec:
					if d.Tok == token.CONST {
						for i, name := range s.Names {
							if len(s.Values) <= i {
								break
							}

							if name.IsExported() {
								log.Debug().Str("const", name.Name).Interface("value", s.Values[i]).Msg("registering basic const")
								constants[pkg][name.Name] = registeredConstant{removePosition(s.Values[i]), removePosition(s.Type), removePosFromCommentGroup(s.Doc)}
							}
						}
					}
				}
			}
		}
	}
}

func registerTypesToDrop(fAST *ast.File) {
	pkg := fAST.Name.Name
	if _, found := typesToDrop[pkg]; !found {
		typesToDrop[pkg] = map[string]bool{}
	}

	for _, decl := range fAST.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if lookupDirective(d.Doc, s.Doc) == mustDrop {
						fmt.Printf("dropping %s.%s\n", pkg, s.Name.Name)
						typesToDrop[pkg][s.Name.Name] = true
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

		log.Debug().Str("rel", rel).Str("file", f.Name()).Msg("parsing file")

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
				dir := lookupDirective(node.Doc)
				if dir == mustKeep {
					return false
				}

				// Should we delete this function declaration if it's unexported or a receiver on an unexported object?
				shouldDelete := !node.Name.IsExported() || dir == mustDrop
				if node.Recv != nil {
					for i, field := range node.Recv.List {
						if ident := typeName(field.Type); ident != nil && (!ident.IsExported() || typesToDrop[f.Name.Name][ident.Name]) {
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
				var genDeclComment *ast.CommentGroup
				if gd, ok := c.Parent().(*ast.GenDecl); ok {
					genDeclComment = gd.Doc
				}

				dir := lookupDirective(node.Doc, node.Comment, genDeclComment)
				keep := node.Name.IsExported() || dir == mustKeep
				if dir == mustDrop || typesToDrop[f.Name.Name][node.Name.Name] {
					keep = false
				}
				if node.Name.Name == "constStr" {
					fmt.Printf("constStr dir %s keep %v %q\n", dir, keep, genDeclComment.Text())
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
							node.Type = typ.node      // replace the type with the actual type
							if typ.docs != nil {
								if parent, ok := c.Parent().(*ast.GenDecl); ok {
									if parent.Doc == nil {
										parent.Doc = typ.docs // copy the docs over
									}
								} else if node.Doc == nil {
									node.Doc = typ.docs // copy the docs over
								}
							}
						}
					}
				}

			case *ast.FuncType:
				return false

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

				dir := lookupDirective(node.Doc, node.Comment)
				if dir == mustKeep {
					keep = true
				} else if dir == mustDrop {
					keep = false
				}

				if !keep {
					c.Delete()
					return false
				}

			case *ast.GenDecl:
				dir := lookupDirective(node.Doc)
				if dir == mustKeep {
					return false
				} else if dir == mustDrop {
					c.Delete()
					return false
				}

				// else if we have a selector, rewrite the package name to see if this is an export from
				// one of our private packages, in which case we want to copy it into the public API
				if node.Tok == token.CONST {
					for _, spec := range node.Specs {
						switch spec := spec.(type) {
						case *ast.ValueSpec:
							for i, value := range spec.Values {
								if selector, ok := value.(*ast.SelectorExpr); ok {
									if pkg, ok := selector.X.(*ast.Ident); ok && constants[pkg.Name] != nil {
										typ, found := constants[pkg.Name][selector.Sel.Name]
										if found {
											spec.Values[i] = typ.node

											if typ.typ != nil {
												// Copy the type over
												spec.Type = typ.typ
											}

											if typ.docs != nil && spec.Doc == nil {
												spec.Doc = typ.docs // copy the docs over
											}
										}
									}
								}
							}
						}
					}
				}

			case *ast.Field:
				// Keep exported fields if there's a doc saying we should
				dir := lookupDirective(node.Doc, node.Comment)
				keep := dir == mustKeep

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

				if dir == mustDrop {
					keep = false
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

	log.Debug().Str("rel", rel).Msg("walking directory")
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
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("unable to get repo directory")
	}
	return string(bytes.TrimSpace(out))
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

type keepDirective string

const (
	none     keepDirective = "none"
	mustKeep               = "keep"
	mustDrop               = "drop"
)

// directiveCache caches the directive for a given comment group,
// as the lookupDirective function mutates the comment which would
// otherwise cause subsequent calls to return a different value.
var directiveCache = make(map[*ast.CommentGroup]keepDirective)

func lookupDirective(nodes ...*ast.CommentGroup) keepDirective {
	result := none
	for _, node := range nodes {
		dir, cached := directiveCache[node]
		if !cached {
			dir = parseDirective(node)
			clearDirectives(node)
			directiveCache[node] = dir
		}

		if dir != none {
			result = dir
		}
	}
	return result
}

func parseDirective(node *ast.CommentGroup) keepDirective {
	if node != nil && node.List != nil {
		for _, comment := range node.List {
			text := strings.TrimSpace(comment.Text)
			switch text {
			case "//publicapigen:keep":
				return mustKeep
			case "//publicapigen:drop":
				return mustDrop
			default:
				continue
			}
		}
	}
	return none
}

func clearDirectives(node *ast.CommentGroup) {
	if node != nil && node.List != nil {
		for i, comment := range node.List {
			text := strings.TrimSpace(comment.Text)
			switch text {
			case "//publicapigen:keep":
			case "//publicapigen:drop":
			default:
				continue
			}
			if i == 0 {
				comment.Text = "  "
			} else {
				comment.Text = "//" // empty comment line as I want the docs to remain active, but I can't remove this without causing a blank line between the comment group and what ever it's associated with
			}
		}
	}
}

func isPrivateFile(fileName string) bool {
	return strings.HasPrefix(fileName, "appruntime/") ||
		strings.Contains(fileName, "internal/") ||
		strings.HasSuffix(fileName, "_internal.go")
}

func removePosFromCommentGroup(doc *ast.CommentGroup) *ast.CommentGroup {
	if doc == nil {
		return nil
	}

	rtn := *doc

	for i, originalLine := range rtn.List {
		line := *originalLine
		line.Slash = token.NoPos
		rtn.List[i] = &line
	}
	return &rtn
}

func removePosition(node ast.Expr) ast.Expr {
	if node == nil {
		return nil
	}

	switch node := node.(type) {
	case *ast.BasicLit:
		lit := *node
		lit.ValuePos = token.NoPos
		return &lit

	case *ast.Ident:
		ident := *node
		ident.NamePos = token.NoPos
		return &ident

	case *ast.SelectorExpr:
		sel := *node
		sel.X = removePosition(sel.X)
		sel.Sel = removePosition(sel.Sel).(*ast.Ident)
		return &sel

	case *ast.UnaryExpr:
		unary := *node
		unary.OpPos = token.NoPos
		unary.X = removePosition(unary.X)
		return &unary

	case *ast.BinaryExpr:
		binary := *node
		binary.OpPos = token.NoPos
		binary.X = removePosition(binary.X)
		binary.Y = removePosition(binary.Y)
		return &binary

	default:
		log.Warn().Interface("node", node).Msg("unhandled node type to remove position from")
		return node
	}
}
