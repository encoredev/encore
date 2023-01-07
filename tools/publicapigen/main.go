package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
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

type parsedFile struct {
	fileName string
	dir      string
	ast      *ast.File
}

var (
	resolvedRepo             = repoDir()
	gitRef                   = repoCommit()
	fset                     = token.NewFileSet()
	formattedFset            = token.NewFileSet()
	files                    = []*parsedFile{}
	commentsToAddToFunctions = map[*ast.File]map[string]*ast.CommentGroup{}
	constants                = map[string]map[string]registeredConstant{}
	types                    = map[string]map[string]registeredType{}
	typesToDrop              = map[string]map[string]bool{}
	usesPanicWrapper         = map[string]bool{} // package dir -> true
)

func main() {
	log.Logger = zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Caller().Logger()
	log.Info().Msg("generating public api")

	// Walk the directory tree and parse all the Go files
	log.Info().Msg("parsing source files...")
	if err := walkDir(filepath.Join(resolvedRepo, "runtime"), "./", readAST); err != nil {
		log.Fatal().Err(err).Msg("unable to walk runtime directory to parse go files")
	}
	slices.SortFunc(files, func(a, b *parsedFile) bool {
		return a.fileName < b.fileName
	})

	// Register all consts and types in our private files, just in case we reference them in the public API
	log.Info().Msg("registering types...")
	for _, f := range files {
		if isPrivateFile(f.fileName) {
			log.Debug().Str("file", f.fileName).Msg("registering types and constants from private implementation files")
			registerTypes(f.fileName, f.ast)
		}
		registerTypesToDrop(f.ast)
	}

	// Then rewrite all the AST to remove implementations
	log.Info().Msg("rewriting ast to remove implementations and unexported items...")
	remaining := make([]*parsedFile, 0, len(files))
	for _, f := range files {
		log.Debug().Str("file", f.fileName).Msg("rewriting ast")
		usesPanic, err := rewriteAST(f.ast)
		if err != nil {
			log.Fatal().Err(err).Str("file", f.fileName).Msg("unable to rewrite ast")
		}

		if len(f.ast.Decls) == 0 && f.ast.Doc == nil {
			log.Debug().Str("file", f.fileName).Msg("removing file as there are no exported decelerations or package comments")
			continue
		}

		if usesPanic {
			usesPanicWrapper[f.dir] = true
		}

		remaining = append(remaining, f)
	}
	files = remaining

	writtenPanicWrapper := make(map[string]bool) // package dir -> true

	// Then write the AST to a file
	outDir := outDir()
	log.Info().Str("out", outDir).Msg("writing public api files...")
	for _, f := range files {
		if isPrivateFile(f.fileName) {
			// "runtime" is a private package as are internal packages
			// any files suffixed with _internal.go are also private and are considered unstable API's
			continue
		}

		log.Debug().Str("file", f.fileName).Msg("writing public api file")

		// Pretty print the file and then re-parse it
		// This repopulates the AST with the comments and formatting
		formattedFile := convertASTToFormattedSrc(fset, f.ast, f.fileName)
		formattedAST, err := parser.ParseFile(formattedFset, f.fileName, formattedFile, parser.ParseComments)
		if err != nil {
			log.Fatal().Err(err).Str("file", f.fileName).Msg("unable to convert back to an AST")
		}

		// Now we can add brand new comments list given set, we
		// can write the help comment into the functions
		writePendingComments(f.ast, formattedAST)

		// Now let's write the file out
		outputFile := filepath.Join(outDir, f.fileName)
		outputDir := filepath.Dir(outputFile)

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatal().Err(err).Str("dir", outputDir).Msg("unable to create output directory")
		}

		if isEmptyFile(formattedAST) {
			continue
		}

		out := convertASTToFormattedSrc(formattedFset, formattedAST, f.fileName)
		if usesPanicWrapper[f.dir] && !writtenPanicWrapper[f.dir] {
			out = append(out, panicWrapperSnippet...)
			writtenPanicWrapper[f.dir] = true
		}

		if err := os.WriteFile(outputFile, out, 0644); err != nil {
			log.Fatal().Err(err).Str("file", f.fileName).Msg("unable to write file")
		}
	}

	if err := buildsSuccessfully(outDir); err != nil {
		log.Fatal().Err(err).Msg("generated code does not build")
	}

	log.Info().Msg("done")
}

func isEmptyFile(f *ast.File) bool {
	return len(f.Decls) == 0 && f.Doc.Text() == ""
}

func convertASTToFormattedSrc(fset *token.FileSet, fAST *ast.File, fileName string) []byte {
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

	return formatted
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
func readAST(path, rel string, file []os.DirEntry) error {
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
		files = append(files, &parsedFile{
			fileName: filepath.Join(rel, f.Name()),
			dir:      rel,
			ast:      fAST,
		})
	}

	return nil
}

func rewriteAST(f *ast.File) (usesPanicWrapper bool, err error) {
	var lastIfaceType *ast.InterfaceType

	astutil.Apply(
		f,
		func(c *astutil.Cursor) bool {
			switch node := c.Node().(type) {
			case *ast.ImportSpec:
				// Drop "_" imports as they are implementation details and not needed in the API contract
				if node.Name != nil && node.Name.Name == "_" {
					c.Delete()
				}
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

					if _, found := commentsToAddToFunctions[f]; !found {
						commentsToAddToFunctions[f] = map[string]*ast.CommentGroup{}
					}

					startLine := fset.Position(start).Line
					endLine := fset.Position(end).Line
					filePath := strings.TrimPrefix(fset.File(node.Pos()).Name(), resolvedRepo)

					commentsToAddToFunctions[f][funcName(node)] = &ast.CommentGroup{
						List: []*ast.Comment{
							{
								Text: "// Encore will provide an implementation to this function at runtime, we do not expose",
							},
							{
								Text: "// the implementation in the API contract as it is an implementation detail, which may change",
							},
							{
								Text: "// between releases.",
							},
							{
								Text: "//",
							},
							{
								Text: "// The current implementation of this function can be found here:",
							},
							{
								Text: "//    https://github.com/encoredev/encore/blob/" + gitRef + filePath + "#L" + strconv.Itoa(startLine) + "-L" + strconv.Itoa(endLine),
							},
						},
					}
				}

				// Drop any parameters that are prefixed with "__" as these are used to indicate that
				// the arguments are added in by Encore's code generators and should be ignored in
				// the customer facing code.
				newFieldList := &ast.FieldList{
					Opening: token.NoPos,
					Closing: token.NoPos,
				}
				for _, p := range node.Type.Params.List {
					if len(p.Names) == 0 || strings.HasPrefix(p.Names[0].Name, "__") {
						continue
					}
					newFieldList.List = append(newFieldList.List, p)
				}
				node.Type.Params = newFieldList

				// If we have any results, add a placeholder names so we can use a naked return.
				results := node.Type.Results
				if results != nil && results.NumFields() > 0 {
					for _, res := range results.List {
						if len(res.Names) == 0 {
							res.Names = []*ast.Ident{ast.NewIdent("_")}
						}
					}
				}

				// If we are keeping the function, replace the implementation with a panic
				// as the code we're generating is only to help the IDE and users understand our API
				// but isn't intended to be used in running apps
				node.Body = &ast.BlockStmt{
					Lbrace: token.NoPos,
					List: []ast.Stmt{
						&ast.ExprStmt{
							X: &ast.CallExpr{
								Fun:    ast.NewIdent("doPanic"),
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
						&ast.ReturnStmt{},
					},
					Rbrace: token.NoPos,
				}
				usesPanicWrapper = true
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

			case *ast.InterfaceType:
				lastIfaceType = node

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

				// Is this field a type list? If so, keep it.
				if bin, ok := node.Type.(*ast.BinaryExpr); ok && bin.Op == token.OR && node.Names == nil {
					keep = true
				}
				// Or is it a type list with a single field within an interface?
				if node.Names == nil && lastIfaceType != nil && posWithin(node.Pos(), lastIfaceType) {
					keep = true
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
	return usesPanicWrapper, nil
}

func writePendingComments(originalFile *ast.File, formattedFile *ast.File) {
	comments, found := commentsToAddToFunctions[originalFile]
	if !found {
		return
	}

	astutil.Apply(formattedFile, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.FuncDecl:
			if comments, found := comments[funcName(node)]; found {
				// check if the body is inline like this:
				//   func Foo() { panic("foo") }
				// and if so, then add a new line to the beginning of the comment
				// to force it to be formatted like:
				//   func Foo() {
				//      // comment
				//      panic("foo")
				//   }
				file := formattedFset.File(node.Pos())
				panicPos := node.Body.List[0].Pos()
				panicLine := file.Line(panicPos)
				if file.LineStart(panicLine) < panicPos-3 {
					comments.List[0].Text = "\n" + comments.List[0].Text
				}

				// Position the comment just before the first expression in the body
				comments.List[0].Slash = panicPos - 1
				formattedFile.Comments = append(formattedFile.Comments, comments)
			}
		}
		return true
	}, nil)

	// The comments _must_ be sorted, otherwise the formatter will get confused
	// and put all out of order comments under the last ordered comment
	sort.SliceStable(formattedFile.Comments, func(i, j int) bool {
		return formattedFile.Comments[i].List[0].Slash < formattedFile.Comments[j].List[0].Slash
	})
}

// walkDir recursively descends path, calling walkFn for directory
func walkDir(dir, rel string, f func(path, rel string, files []os.DirEntry) error) error {
	if rel == "types/uuid" {
		// we don't want to rewrite this package
		return nil
	}

	log.Debug().Str("rel", rel).Msg("walking directory")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Split the files and dirs
	var dirs, files []os.DirEntry
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

func repoCommit() string {
	// First check if we have a tag pointed at this commit
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	cmd.Dir = resolvedRepo
	out, err := cmd.CombinedOutput()

	// If that doesn't work, then just get the commit hash
	if err != nil || string(bytes.TrimSpace(out)) == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = resolvedRepo
		out, err := cmd.CombinedOutput()
		if err != nil {
			panic("unable to get repo commit")
		}
		return string(bytes.TrimSpace(out))
	}

	parts := strings.Split(string(bytes.TrimSpace(out)), "\n")
	return strings.TrimSpace(parts[0])
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

func funcName(node *ast.FuncDecl) string {
	var name strings.Builder

	if node.Recv != nil {
		name.WriteRune('(')
		typ := node.Recv.List[0].Type

		if star, ok := typ.(*ast.StarExpr); ok {
			typ = star.X
			name.WriteRune('*')
		}

		if index, ok := typ.(*ast.IndexExpr); ok {
			name.WriteString(index.X.(*ast.Ident).Name)
			name.WriteString("[]")
		} else if indexList, ok := typ.(*ast.IndexListExpr); ok {
			name.WriteString(indexList.X.(*ast.Ident).Name)
			name.WriteString("[]")
		} else {
			name.WriteString(typ.(*ast.Ident).Name)
		}

		name.WriteString(").")
	}

	name.WriteString(node.Name.Name)
	return name.String()
}

const panicWrapperSnippet = `
// doPanic is a wrapper around panic to prevent static analysis tools
// from thinking Encore APIs unconditionally panic.,
func doPanic(v any) {
	if true {
		panic(v)
	}
}
`

func posWithin(pos token.Pos, node ast.Node) bool {
	return pos >= node.Pos() && pos < node.End()
}

func buildsSuccessfully(dir string) error {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("'go build' failed: %v: %s", err, out)
	}
	return err
}
