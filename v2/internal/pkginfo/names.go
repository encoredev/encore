package pkginfo

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"strconv"
	"strings"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
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
					scope.Insert(d.Name.Name, &IdentInfo{Package: true})
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
					case *ast.TypeSpec:
						doc = spec.Doc.Text()
					}
					if doc == "" && len(d.Specs) == 1 {
						doc = d.Doc.Text()
					}

					switch spec := spec.(type) {
					case *ast.ImportSpec:
						// Skip, file-level
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							scope.Insert(name.Name, &IdentInfo{Package: true})
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
						scope.Insert(spec.Name.Name, &IdentInfo{Package: true})
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
			file:       f,
			nameToPath: make(map[string]paths.Pkg),
			idents:     make(map[*ast.Ident]*IdentInfo),
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
				dstPkgPath = r.pkg.ImportPath.JoinSlash(strPath)
			} else {
				dstPkgPath, ok = paths.PkgPath(strPath)
				if !ok {
					r.l.c.Errs.Addf(pos, "invalid import path %q", strPath)
					continue
				}
			}

			pkg := r.l.MustLoadPkg(pos, dstPkgPath)
			localName := pkg.Name
			if is.Name != nil {
				if is.Name.Name == "." {
					// TODO(andre) handle this
					r.l.c.Errs.Fatalf(pos, "dot imports are currently unsupported by Encore's static analysis")
					continue
				}
				localName = is.Name.Name
			}

			// Add the name as long as it's not "_".
			if localName != "_" {
				if p2 := r.res.nameToPath[localName]; p2 != "" {
					r.l.c.Errs.Addf(pos, "name %s already declared (import of package %s)", localName, p2)
					continue
				}
				r.res.nameToPath[localName] = dstPkgPath
			}

		}
	}
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
				r.scope.Insert(name.Name, &IdentInfo{Local: true})
			}
		}
	}
	for _, field := range fd.Type.Params.List {
		for _, name := range field.Names {
			r.scope.Insert(name.Name, &IdentInfo{Local: true})
		}
	}
	if fd.Type.Results != nil {
		for _, field := range fd.Type.Results.List {
			for _, name := range field.Names {
				r.scope.Insert(name.Name, &IdentInfo{Local: true})
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
				r.define(id, &IdentInfo{Local: true})
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
					r.define(name, &IdentInfo{Local: true})
				}
			case *ast.TypeSpec:
				r.expr(spec.Type)
				r.define(spec.Name, &IdentInfo{Local: true})
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
				r.define(name, &IdentInfo{Local: true})
			}
		}
		if expr.Type.Results != nil {
			for _, field := range expr.Type.Results.List {
				for _, name := range field.Names {
					r.define(name, &IdentInfo{Local: true})
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
		r.res.calls = append(r.res.calls, expr)
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
	if obj := r.scope.LookupParent(id.Name); obj != nil {
		r.res.idents[id] = obj
	} else if path := r.res.nameToPath[id.Name]; path != "" {
		r.res.idents[id] = &IdentInfo{
			ImportPath: path,
		}
	}
}

func (r *fileNameResolver) define(id *ast.Ident, name *IdentInfo) {
	r.res.idents[id] = name
	r.scope.Insert(id.Name, name)
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

func (n *PkgNames) GoString() string {
	return "&pkginfo.PkgNames{...}"
}

// FileNames contains name resolution results for a single file.
type FileNames struct {
	file       *File                     // file it belongs to
	nameToPath map[string]paths.Pkg      // local name -> path
	idents     map[*ast.Ident]*IdentInfo // ident -> resolved
	calls      []*ast.CallExpr
}

func (n *FileNames) GoString() string {
	return "&pkginfo.FileNames{...}"
}

// ResolvePkgPath resolves the package path a given identifier name
// resolves to.
func (f *FileNames) ResolvePkgPath(name string) (pkgPath paths.Pkg, ok bool) {
	pkgPath, ok = f.nameToPath[name]
	return pkgPath, ok
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
		if name := f.idents[node]; name != nil && name.Package {
			return QualifiedName{f.file.Pkg.ImportPath, node.Name}, true
		}
	case *ast.SelectorExpr:
		// If it's a selector, then we're looking for something which has been imported from another package
		if pkgName, ok := node.X.(*ast.Ident); ok {
			if resolvedIdent := f.idents[pkgName]; resolvedIdent != nil && resolvedIdent.ImportPath != "" {
				return QualifiedName{resolvedIdent.ImportPath, node.Sel.Name}, true
			}
		}
	}

	return QualifiedName{}, false
}

// IdentInfo provides metadata for a single identifier.
type IdentInfo struct {
	Package    bool      // package symbol
	Local      bool      // locally defined symbol
	ImportPath paths.Pkg // non-zero indicates it resolves to the package with the given import path
}

// scope maps names to information about them.
type scope struct {
	names  map[string]*IdentInfo
	parent *scope
}

func newScope(parent *scope) *scope {
	return &scope{
		names:  make(map[string]*IdentInfo),
		parent: parent,
	}
}

func (s *scope) Pop() *scope {
	return s.parent
}

func (s *scope) Insert(name string, r *IdentInfo) {
	if name != "_" {
		s.names[name] = r
	}
}

func (s *scope) Lookup(name string) *IdentInfo {
	return s.names[name]
}

func (s *scope) LookupParent(name string) *IdentInfo {
	if r := s.names[name]; r != nil {
		return r
	} else if s.parent != nil {
		return s.parent.LookupParent(name)
	}
	return nil
}
