package pkginfo

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/agnivade/levenshtein"

	"encr.dev/pkg/fns"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
)

// resolvePkgNames resolves package-level names for the given package.
func resolvePkgNames(pkg *Package) *PkgNames {
	decls := make(map[string]*PkgDeclInfo)
	scope := newScope(nil)

	for _, f := range pkg.Files {
		for _, d := range f.AST().Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				// HACK(andre) If the RPC was defined as a method on a service struct we
				// generate a synthetic package-level func as part of the user-facing code generation.
				// This happens after parsing, so at the parsing phase we ignore the user-facing code generation.
				//
				// To properly parse code that references those package-level funcs, register
				// service struct-based APIs as existing with synthetic package-level identifiers.
				isServiceStructAPI := d.Recv != nil && isEncoreAPI(d)

				if d.Recv == nil || isServiceStructAPI {
					scope.Insert(d.Name.Name, pkgObject)
					decls[d.Name.Name] = &PkgDeclInfo{
						Name: d.Name.Name,
						File: f,
						Pos:  d.Name.Pos(),
						Doc:  d.Doc.Text(),
						Type: token.FUNC,
						Func: d,
					}
				}

			case *ast.GenDecl:
				for _, spec := range d.Specs {
					var doc string
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						doc = spec.Doc.Text()
						if doc == "" {
							doc = spec.Comment.Text()
						}
					case *ast.TypeSpec:
						doc = spec.Doc.Text()
						if doc == "" {
							doc = spec.Comment.Text()
						}
					}
					if doc == "" && len(d.Specs) == 1 {
						doc = d.Doc.Text()
					}

					switch spec := spec.(type) {
					case *ast.ImportSpec:
						// Skip, file-level
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							scope.Insert(name.Name, pkgObject)
							decls[name.Name] = &PkgDeclInfo{
								Name:    name.Name,
								File:    f,
								Pos:     name.Pos(),
								Doc:     doc,
								Type:    d.Tok,
								Spec:    spec,
								GenDecl: d,
							}
						}
					case *ast.TypeSpec:
						scope.Insert(spec.Name.Name, pkgObject)
						decls[spec.Name.Name] = &PkgDeclInfo{
							Name:    spec.Name.Name,
							File:    f,
							Pos:     spec.Name.Pos(),
							Doc:     doc,
							Type:    d.Tok,
							GenDecl: d,
							Spec:    spec,
						}
					}
				}
			}
		}
	}

	return &PkgNames{
		PkgDecls: decls,
		pkgScope: scope,
	}
}

// fileNameResolver resolves file-local names within a package.
type fileNameResolver struct {
	l   *Loader
	f   *File
	pkg *Package
	tr  *parsectx.TraceLogger

	// res is the resulting name information.
	res *FileNames

	// scope is the current scope being processed.
	scope *scope
}

func resolveFileNames(f *File) *FileNames {
	tr := f.l.c.Trace("pkgload.resolveFileNames", "pkg", f.Pkg.ImportPath, "file", f.Name)
	defer tr.Done()
	r := &fileNameResolver{
		l:  f.l,
		f:  f,
		tr: tr,
		res: &FileNames{
			loader:           f.l,
			file:             f,
			idents:           make(map[*ast.Ident]identKind),
			knownImportNames: make(map[string]*importedPkg),
		},
		scope: f.Pkg.Names().pkgScope,
	}

	r.processImports()
	for _, decl := range f.AST().Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						r.ident(name)
					}
					r.exprList(spec.Values)
					r.expr(spec.Type)
				case *ast.TypeSpec:
					r.expr(spec.Type)
					r.expr(spec.Type)
				}
			}

		case *ast.FuncDecl:
			r.funcDecl(decl)
		}
	}

	return r.res
}

// processImports finds the file-local names of imports we care about.
// The name mapping is stored in r.File.PathToName and r.File.NameToPath.
func (r *fileNameResolver) processImports() {
	for _, decl := range r.f.AST().Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gd.Specs {
			is := spec.(*ast.ImportSpec)
			pos := is.Path.Pos()

			strPath, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				r.l.c.Errs.Addf(pos, "invalid import path %s", is.Path.Value)
				continue
			}

			var dstPkgPath paths.Pkg
			if build.IsLocalImport(strPath) {
				dstPkgPath = r.pkg.ImportPath.JoinSlash(paths.RelSlash(strPath))
			} else {
				dstPkgPath, ok = paths.PkgPath(strPath)
				if !ok {
					r.l.c.Errs.Addf(pos, "invalid import path %q", strPath)
					continue
				}
			}

			// Do we have a local name?
			localName := ""
			if is.Name != nil {
				if is.Name.Name == "." {
					// TODO(andre) handle this
					r.l.c.Errs.Fatalf(pos, "dot imports are currently unsupported by Encore's static analysis")
					continue
				}
				localName = is.Name.Name
			}

			// Track this import as long as the local name is not "_".
			if localName != "_" {
				// localName is generally "" if the package was imported without
				// giving it an explicit alias (like `import foo "path/to/foo"`).
				// If that's the case, attempt to classify the package name.
				if localName == "" {
					localName = r.resolveKnownPkgName(dstPkgPath)
				}

				r.res.imports = append(r.res.imports, &importedPkg{
					importPath:      dstPkgPath,
					lastPathSegment: path.Base(dstPkgPath.String()),
					localName:       localName,
				})
			}

		}
	}
}

// resolveKnownPkgName returns the known package name for the given import path, if any.
// It uses the fact that the encore.dev module and the standard library guarantee
// that the last path segment is the package name.
//
// If the name is not known it reports "".
func (r *fileNameResolver) resolveKnownPkgName(pkgPath paths.Pkg) (name string) {
	stdlib := paths.StdlibMod()
	encoreRuntime := r.l.runtimeModule.Path
	if stdlib.LexicallyContains(pkgPath) || encoreRuntime.LexicallyContains(pkgPath) {
		return path.Base(pkgPath.String())
	}
	return ""
}

func (r *fileNameResolver) funcDecl(fd *ast.FuncDecl) {
	r.openScope()
	defer r.closeScope()

	// First resolve types before introducing names
	for _, param := range fd.Type.Params.List {
		r.expr(param.Type)
	}
	if fd.Type.Results != nil {
		for _, result := range fd.Type.Results.List {
			r.expr(result.Type)
		}
	}

	if fd.Recv != nil {
		for _, field := range fd.Recv.List {
			for _, name := range field.Names {
				r.scope.Insert(name.Name, localObject)
			}
		}
	}
	for _, field := range fd.Type.Params.List {
		for _, name := range field.Names {
			r.scope.Insert(name.Name, localObject)
		}
	}
	if fd.Type.Results != nil {
		for _, field := range fd.Type.Results.List {
			for _, name := range field.Names {
				r.scope.Insert(name.Name, localObject)
			}
		}
	}
	if fd.Body != nil {
		r.stmtList(fd.Body.List)
	}
}

func (r *fileNameResolver) stmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		r.exprList(stmt.Rhs)
		for _, lhs := range stmt.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && stmt.Tok == token.DEFINE {
				r.define(id, localObject)
			}
		}

	case *ast.BlockStmt:
		r.openScope()
		defer r.closeScope()
		r.stmtList(stmt.List)

	case *ast.DeclStmt:
		decl := stmt.Decl.(*ast.GenDecl)
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.ValueSpec:
				r.exprList(spec.Values)
				r.expr(spec.Type)
				for _, name := range spec.Names {
					r.define(name, localObject)
				}
			case *ast.TypeSpec:
				r.expr(spec.Type)
				r.define(spec.Name, localObject)
			}
		}

	case *ast.DeferStmt:
		r.expr(stmt.Call)

	case *ast.ExprStmt:
		r.expr(stmt.X)

	case *ast.ForStmt:
		r.openScope()
		defer r.closeScope()
		r.stmt(stmt.Init)
		r.expr(stmt.Cond)
		r.stmt(stmt.Post)
		r.stmt(stmt.Body)

	case *ast.GoStmt:
		r.expr(stmt.Call)

	case *ast.IfStmt:
		r.openScope()
		defer r.closeScope()
		r.stmt(stmt.Init)
		r.expr(stmt.Cond)
		r.stmt(stmt.Body)
		r.stmt(stmt.Else)

	case *ast.IncDecStmt:
		r.expr(stmt.X)

	case *ast.LabeledStmt:
		r.stmt(stmt.Stmt)

	case *ast.RangeStmt:
		r.openScope()
		defer r.closeScope()
		r.expr(stmt.X)
		r.expr(stmt.Key)
		r.expr(stmt.Value)
		r.stmt(stmt.Body)

	case *ast.ReturnStmt:
		r.exprList(stmt.Results)

	case *ast.SelectStmt:
		r.stmtList(stmt.Body.List)

	case *ast.SendStmt:
		r.expr(stmt.Value)
		r.expr(stmt.Chan)

	case *ast.SwitchStmt:
		r.openScope()
		defer r.closeScope()
		r.stmt(stmt.Init)
		r.expr(stmt.Tag)
		r.stmt(stmt.Body)

	case *ast.TypeSwitchStmt:
		r.openScope()
		defer r.closeScope()
		r.stmt(stmt.Init)
		r.stmt(stmt.Assign)
		r.stmt(stmt.Body)

	case *ast.CommClause:
		r.openScope()
		defer r.closeScope()
		r.stmt(stmt.Comm)
		r.stmtList(stmt.Body)

	case *ast.CaseClause:
		r.exprList(stmt.List)
		r.openScope()
		defer r.closeScope()
		r.stmtList(stmt.Body)

	case *ast.BadStmt, *ast.BranchStmt, *ast.EmptyStmt:
		// do nothing

	default:
		panic(fmt.Sprintf("unhandled ast.Stmt type: %T", stmt))
	}
}

func (r *fileNameResolver) expr(expr ast.Expr) {
	switch expr := expr.(type) {
	case nil:
		// do nothing

	case *ast.Ident:
		r.ident(expr)

	case *ast.Ellipsis:
		r.expr(expr.Elt)

	case *ast.FuncLit:
		r.openScope()
		defer r.closeScope()

		// First resolve types before introducing names
		for _, param := range expr.Type.Params.List {
			r.expr(param.Type)
		}
		if expr.Type.Results != nil {
			for _, result := range expr.Type.Results.List {
				r.expr(result.Type)
			}
		}

		for _, field := range expr.Type.Params.List {
			for _, name := range field.Names {
				r.define(name, localObject)
			}
		}
		if expr.Type.Results != nil {
			for _, field := range expr.Type.Results.List {
				for _, name := range field.Names {
					r.define(name, localObject)
				}
			}
		}
		if expr.Body != nil {
			r.stmt(expr.Body)
		}

	case *ast.CompositeLit:
		r.expr(expr.Type)
		r.exprList(expr.Elts)

	case *ast.ParenExpr:
		r.expr(expr.X)

	case *ast.SelectorExpr:
		r.expr(expr.X)
		// Note: we don't treat 'Foo' in 'x.Foo' as an identifier,
		// as it does not introduce a new name to any scope.

	case *ast.IndexExpr:
		r.expr(expr.X)
		r.expr(expr.Index)

	case *ast.IndexListExpr:
		// An IndexListExpr node represents an expression followed by multiple indices.
		// e.g. `X[A, B, C]` or `X[1, 2]`
		r.expr(expr.X)
		for _, index := range expr.Indices {
			r.expr(index)
		}

	case *ast.SliceExpr:
		r.expr(expr.X)
		r.expr(expr.Low)
		r.expr(expr.High)
		r.expr(expr.Max)

	case *ast.TypeAssertExpr:
		r.expr(expr.X)
		r.expr(expr.Type)

	case *ast.CallExpr:
		r.expr(expr.Fun)
		r.exprList(expr.Args)

	case *ast.StarExpr:
		r.expr(expr.X)

	case *ast.UnaryExpr:
		r.expr(expr.X)

	case *ast.BinaryExpr:
		r.expr(expr.X)
		r.expr(expr.Y)

	case *ast.KeyValueExpr:
		// HACK: We want to track uses of functions. This is tricky because
		// struct types use keys that are idents that refer to the struct field,
		// while map types can use keys to refer to idents in scope.
		//
		// Unfortunately We cannot easily know the type of the composite literal
		// without typechecking. However, funcs are incomparable and therefore
		// are not valid as map keys. So let's simply avoid tracking idents
		// in the keys, and rely on the compiler to eventually catch this for us.
		if _, ok := expr.Key.(*ast.Ident); !ok {
			r.expr(expr.Key)
		}
		r.expr(expr.Value)

	case *ast.ArrayType:
		r.expr(expr.Len)
		r.expr(expr.Elt)

	case *ast.StructType:
		for _, field := range expr.Fields.List {
			r.expr(field.Type)
			// Don't look at names; they don't resolve to outside scope
		}

	case *ast.FuncType:
		for _, field := range expr.Params.List {
			r.expr(field.Type)
			// Don't look at names; they don't resolve to outside scope
		}
		if expr.Results != nil {
			for _, field := range expr.Results.List {
				r.expr(field.Type)
				// Don't look at names; they don't resolve to outside scope
			}
		}

	case *ast.InterfaceType:
		for _, field := range expr.Methods.List {
			r.expr(field.Type)
			// Don't look at names; they don't resolve to outside scope
		}

	case *ast.MapType:
		r.expr(expr.Key)
		r.expr(expr.Value)

	case *ast.ChanType:
		r.expr(expr.Value)

	case *ast.BadExpr, *ast.BasicLit:
		// do nothing

	default:
		// If we don't process this then return false
		panic(fmt.Sprintf("unhandled ast.Expr type: %T", expr))
	}
}

func (r *fileNameResolver) stmtList(stmts []ast.Stmt) {
	for _, s := range stmts {
		r.stmt(s)
	}
}

func (r *fileNameResolver) exprList(exprs []ast.Expr) {
	for _, x := range exprs {
		r.expr(x)
	}
}

func (r *fileNameResolver) ident(id *ast.Ident) {
	// Map this ident. If the name is already in scope, use that definition.
	// Otherwise check if it's an imported name.
	if kind := r.scope.LookupParent(id.Name); kind != none {
		r.res.idents[id] = kind
	} else {
		r.res.idents[id] = importName
	}
}

func (r *fileNameResolver) define(id *ast.Ident, kind identKind) {
	r.res.idents[id] = kind
	r.scope.Insert(id.Name, kind)
}

func (r *fileNameResolver) openScope() {
	r.scope = newScope(r.scope)
}

func (r *fileNameResolver) closeScope() {
	r.scope = r.scope.Pop()
}

func isEncoreAPI(fd *ast.FuncDecl) bool {
	fd.Doc.Text()
	if fd.Doc == nil {
		return false
	}

	const directive = "encore:api"
	for _, c := range fd.Doc.List {
		if strings.HasPrefix(c.Text, "//"+directive) {
			return true
		}
	}

	// Legacy syntax
	lines := strings.Split(fd.Doc.Text(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, directive) {
			return true
		}
	}

	return false
}

// PkgDeclInfo provides metadata for a package-level declaration.
type PkgDeclInfo struct {
	Name string
	File *File
	Pos  token.Pos
	Doc  string

	// Type describes what type of declaration this is.
	// It's one of CONST, TYPE, VAR, or FUNC.
	Type token.Token

	// Spec is the spec for this declaration and GenDecl the declaration block
	// it belongs to. They are set only when Type != FUNC.
	Spec    ast.Spec
	GenDecl *ast.GenDecl

	// Func is the function declaration, if Type == FUNC.
	Func *ast.FuncDecl // for Type == FUNC
	// Recv is the receiver type, if Type == FUNC and the function is a method.
	Recv *PkgDeclInfo
}

// PkgNames contains name information that's package-global.
type PkgNames struct {
	// PkgDecls contains package-level declarations, keyed by name.
	PkgDecls map[string]*PkgDeclInfo

	// Funcs are all the func declarations.
	Funcs []*PkgDeclInfo

	// pkgScope tracks the scope information for the package scope.
	// It's stored to avoid having to recompute it when querying
	// for individual files' file-level name information.
	pkgScope *scope
}

func (n *PkgNames) FuncDecl(name string) option.Option[*ast.FuncDecl] {
	if fn, ok := n.PkgDecls[name]; ok && fn.Type == token.FUNC {
		return option.Some(fn.Func)
	}
	return option.None[*ast.FuncDecl]()
}

func (n *PkgNames) GoString() string {
	return "&pkginfo.PkgNames{...}"
}

// FileNames contains name resolution results for a single file.
type FileNames struct {
	loader *Loader // the loader this file comes from

	// file is the file the names belong to.
	file *File

	// idents maps identifiers in the file to information about them.
	idents map[*ast.Ident]identKind

	imports []*importedPkg

	// knownImportNames tracks the resolved import names for this file.
	knownImportNamesMu sync.Mutex
	knownImportNames   map[string]*importedPkg

	//// localNameToPkg maps local names to the package they refer to.
	//localNameToPkg map[string]*importedPkg
	//
	//
	//// nameToPath contains resolved local name -> import
	//nameToPath map[string]paths.Pkg // local name -> path
}

func (n *FileNames) GoString() string {
	return "&pkginfo.FileNames{...}"
}

// ResolvePkgPath resolves the package path a given identifier name
// resolves to.
func (f *FileNames) ResolvePkgPath(cause token.Pos, name string) (pkgPath paths.Pkg, ok bool) {
	return f.resolveImportPath(cause, name)
}

// ResolvePkgLevelRef resolves the node to the package-level reference it refers to.
// Expr must be either *ast.Ident or *ast.SelectorExpr.
// If it doesn't refer to a package-level reference it returns ok == false.
func (f *FileNames) ResolvePkgLevelRef(expr ast.Expr) (name QualifiedName, ok bool) {
	// Unwrap type arguments
	switch x := expr.(type) {
	case *ast.IndexExpr:
		expr = x.X
	case *ast.IndexListExpr:
		expr = x.X
	}

	// Resolve the identifier
	switch node := expr.(type) {
	case *ast.Ident:
		// If it's an ident, then we're looking for something which resolves to a package-level object defined
		// in the same package as the ident is located in.
		if f.idents[node] == pkgObject {
			return QualifiedName{f.file.Pkg.ImportPath, node.Name}, true
		}
	case *ast.SelectorExpr:
		// If it's a selector, then we're looking for something which has been imported from another package
		if pkgName, ok := node.X.(*ast.Ident); ok {
			if f.idents[pkgName] == importName {
				if importPath, ok := f.resolveImportPath(expr.Pos(), pkgName.Name); ok {
					return QualifiedName{importPath, node.Sel.Name}, true
				}
			}
		}
	}

	return QualifiedName{}, false
}

func (f *FileNames) resolveImportPath(cause token.Pos, identName string) (pkgPath paths.Pkg, ok bool) {
	// Do we already have this name resolved?
	f.knownImportNamesMu.Lock()
	pkg := f.knownImportNames[identName]
	f.knownImportNamesMu.Unlock()
	if pkg != nil {
		return pkg.importPath, true
	}

	// Otherwise, resolve it. We do this by looking at the imported packages
	// and determine which one is most likely to be the one we're looking for.

	resolvePkg := func() *importedPkg {
		// First look for an explicit import alias.
		for _, pkg := range f.imports {
			if pkg.localName == identName {
				return pkg
			}
		}

		// Use heuristics to guess the package.

		// Bucket the packages into a group of exact matches
		// and everything else.
		var (
			exactMatches []*importedPkg
			otherPkgs    = make([]*importedPkg, 0, len(f.imports))
		)
		for _, pkg := range f.imports {
			// If there is an explicit local name, ignore the package
			// since we've already checked those above.
			if pkg.localName != "" {
				continue
			}

			if pkg.lastPathSegment == identName {
				exactMatches = append(exactMatches, pkg)
			} else {
				otherPkgs = append(otherPkgs, pkg)
			}
		}

		processGroup := func(pkgs []*importedPkg) *importedPkg {
			for _, pkg := range pkgs {
				// Load the package to determine if it has the name we're looking for.
				pkg.loadOnce.Do(func() {
					pkg.loadedPkg = f.loader.MustLoadPkg(cause, pkg.importPath)
				})

				if pkg.loadedPkg.Name == identName {
					// We've found the package we're looking for.
					return pkg
				}
			}
			return nil
		}

		// Check the exact matches first.
		if pkg := processGroup(exactMatches); pkg != nil {
			return pkg
		}

		// If not, check the remaining packages. Sort them
		// by levenshtein distance to start with the likeliest
		// candidates first.
		distances := fns.Map(otherPkgs, func(pkg *importedPkg) int {
			return levenshtein.ComputeDistance(pkg.lastPathSegment, identName)
		})
		sort.Slice(otherPkgs, func(i, j int) bool {
			return distances[i] < distances[j]
		})
		return processGroup(otherPkgs)
	}

	if pkg := resolvePkg(); pkg != nil {
		f.knownImportNamesMu.Lock()
		f.knownImportNames[identName] = pkg
		f.knownImportNamesMu.Unlock()
		return pkg.importPath, true
	}

	return "", false
}

type identKind string

const (
	// none is a none identKind.
	none identKind = ""

	// pkgObject is used for identifiers pointing to package-level
	// objects in the same package as the identifier.
	pkgObject identKind = "pkgObject"

	// localObject is used for identifiers pointing to
	// function-local objects.
	localObject identKind = "localObject"

	// importName is used for identifiers pointing to
	// an imported package.
	importName identKind = "importName"
)

// importedPkg represents a package that has been imported
// in a file.
//
// Since the local name is the package name, which can
// be different from the last element of the import path, we
// can't know for sure which package it refers to until
// we've parsed that package.
type importedPkg struct {
	importPath paths.Pkg

	// lastPathSegment is the last segment of the import path.
	// It's used as a heuristic to guess which package to
	// parse first.
	lastPathSegment string

	// localName is the name of the package in the file.
	localName string

	// loadedPkg is the loaded package, if any.
	// It must be accessed using the sync.Once.
	loadOnce  sync.Once
	loadedPkg *Package
}

// scope maps names to information about them.
type scope struct {
	names  map[string]identKind
	parent *scope
}

func newScope(parent *scope) *scope {
	return &scope{
		names:  make(map[string]identKind),
		parent: parent,
	}
}

func (s *scope) Pop() *scope {
	return s.parent
}

func (s *scope) Insert(name string, kind identKind) {
	if name != "_" {
		s.names[name] = kind
	}
}

func (s *scope) Lookup(name string) identKind {
	return s.names[name]
}

func (s *scope) LookupParent(name string) identKind {
	if kind := s.names[name]; kind != none {
		return kind
	} else if s.parent != nil {
		return s.parent.LookupParent(name)
	}
	return none
}
