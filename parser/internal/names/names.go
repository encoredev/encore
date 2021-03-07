package names

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/scanner"
	"go/token"
	pathpkg "path"
	"strconv"

	"encr.dev/parser/est"
)

// TrackedPackages defines the set of packages to track,
// defined as a map from import path to package name.
//
// It is used to allow Resolve to efficiently determine
// the name of a package given its import path, as one
// otherwise needs to parse the package source to find it.
type TrackedPackages map[string]string

// Resolution represents the name resolution results.
type Resolution struct {
	Decls map[string]*PkgDecl // package-level declarations, keyed by name
	Files map[*est.File]*File
}

// PkgDecl provides metadata for a package-level declaration.
type PkgDecl struct {
	Name string
	File *est.File
	Pos  token.Pos
	Type token.Token   // CONST, TYPE, VAR, FUNC
	Func *ast.FuncDecl // for Type == FUNC
	Spec ast.Spec      // for other types
	Doc  string
}

// File provides file-level name resolution results.
type File struct {
	PathToName map[string]string    // path -> local name
	NameToPath map[string]string    // local name -> path
	Idents     map[*ast.Ident]*Name // ident -> resolved
	Calls      []*ast.CallExpr
}

// Name provides metadata for a single identifier.
type Name struct {
	Package    bool   // package symbol
	Local      bool   // locally defined symbol
	ImportPath string // non-zero indicates it resolves to the package with the given import path
}

// Resolve resolves information about the names (idents) in the given package.
// The reported error is of type scanner.ErrorList if non-nil.
func Resolve(fset *token.FileSet, track TrackedPackages, pkg *est.Package) (*Resolution, error) {
	res := &Resolution{
		Decls: make(map[string]*PkgDecl),
		Files: make(map[*est.File]*File),
	}

	pkgNames, pkgScope := collectPackageObjects(pkg)
	res.Decls = pkgNames

	var errors scanner.ErrorList
	for _, file := range pkg.Files {
		f := &File{
			PathToName: make(map[string]string),
			NameToPath: make(map[string]string),
			Idents:     make(map[*ast.Ident]*Name),
		}
		res.Files[file] = f
		r := resolver{
			File:  f,
			fset:  fset,
			track: track,
			scope: pkgScope,
		}
		if err := r.Process(file); err != nil {
			if e, ok := err.(*scanner.Error); ok {
				errors = append(errors, e)
			} else {
				errors.Add(fset.Position(file.AST.Pos()), err.Error())
			}
		}
	}
	return res, errors.Err()
}

// collectPackageObjects collects all package-level objects from the given files.
func collectPackageObjects(pkg *est.Package) (map[string]*PkgDecl, *scope) {
	decls := make(map[string]*PkgDecl)
	scope := newScope(nil)

	for _, f := range pkg.Files {
		for _, d := range f.AST.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					scope.Insert(d.Name.Name, &Name{Package: true})
					decls[d.Name.Name] = &PkgDecl{
						Name: d.Name.Name,
						File: f,
						Pos:  d.Name.Pos(),
						Type: token.FUNC,
						Func: d,
						Doc:  d.Doc.Text(),
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
							scope.Insert(name.Name, &Name{Package: true})
							decls[name.Name] = &PkgDecl{
								Name: name.Name,
								File: f,
								Pos:  name.Pos(),
								Type: d.Tok,
								Spec: spec,
								Doc:  doc,
							}
						}
					case *ast.TypeSpec:
						scope.Insert(spec.Name.Name, &Name{Package: true})
						decls[spec.Name.Name] = &PkgDecl{
							Name: spec.Name.Name,
							File: f,
							Pos:  spec.Name.Pos(),
							Type: d.Tok,
							Spec: spec,
							Doc:  doc,
						}
					}
				}
			}
		}
	}
	return decls, scope
}

type resolver struct {
	*File
	pkg   *est.Package
	fset  *token.FileSet
	track TrackedPackages
	scope *scope
}

func (r *resolver) Process(f *est.File) error {
	if err := r.processImports(f); err != nil {
		return err
	}
	for _, decl := range f.AST.Decls {
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
	return nil
}

// processImports finds the file-local names of imports we care about.
// The name mapping is stored in r.File.PathToName and r.File.NameToPath.
func (r *resolver) processImports(f *est.File) error {
	for _, decl := range f.AST.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gd.Specs {
			is := spec.(*ast.ImportSpec)
			path, err := strconv.Unquote(is.Path.Value)
			if err != nil {
				return &scanner.Error{
					Pos: r.fset.Position(is.Path.Pos()),
					Msg: "invalid import path " + is.Path.Value,
				}
			}
			if build.IsLocalImport(path) {
				path = pathpkg.Join(r.pkg.ImportPath, path)
			}
			if pkgName := r.track[path]; pkgName != "" {
				if is.Name != nil {
					if is.Name.Name == "." {
						return &scanner.Error{
							Pos: r.fset.Position(is.Name.Pos()),
							Msg: "cannot use dot import of Encore-related packages",
						}
					}
					pkgName = is.Name.Name
				}
				if p2 := r.NameToPath[pkgName]; p2 != "" {
					return &scanner.Error{
						Pos: r.fset.Position(is.Path.Pos()),
						Msg: fmt.Sprintf("name %s already declared (import of package %s)", pkgName, p2),
					}
				}
				r.PathToName[path] = pkgName
				r.NameToPath[pkgName] = path
			}
		}
	}
	return nil
}

func (r *resolver) funcDecl(fd *ast.FuncDecl) {
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
				r.scope.Insert(name.Name, &Name{Local: true})
			}
		}
	}
	for _, field := range fd.Type.Params.List {
		for _, name := range field.Names {
			r.scope.Insert(name.Name, &Name{Local: true})
		}
	}
	if fd.Type.Results != nil {
		for _, field := range fd.Type.Results.List {
			for _, name := range field.Names {
				r.scope.Insert(name.Name, &Name{Local: true})
			}
		}
	}
	if fd.Body != nil {
		r.stmtList(fd.Body.List)
	}
}

func (r *resolver) stmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch stmt := stmt.(type) {
	case *ast.AssignStmt:
		r.exprList(stmt.Rhs)
		for _, lhs := range stmt.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && stmt.Tok == token.DEFINE {
				r.define(id, &Name{Local: true})
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
					r.define(name, &Name{Local: true})
				}
			case *ast.TypeSpec:
				r.expr(spec.Type)
				r.define(spec.Name, &Name{Local: true})
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

func (r *resolver) expr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch expr := expr.(type) {
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
				r.define(name, &Name{Local: true})
			}
		}
		if expr.Type.Results != nil {
			for _, field := range expr.Type.Results.List {
				for _, name := range field.Names {
					r.define(name, &Name{Local: true})
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
		r.expr(expr.Sel)

	case *ast.IndexExpr:
		r.expr(expr.X)
		r.expr(expr.Index)

	case *ast.SliceExpr:
		r.expr(expr.X)
		r.expr(expr.Low)
		r.expr(expr.High)
		r.expr(expr.Max)

	case *ast.TypeAssertExpr:
		r.expr(expr.X)
		r.expr(expr.Type)

	case *ast.CallExpr:
		r.Calls = append(r.Calls, expr)
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
		panic(fmt.Sprintf("unhandled ast.Expr type: %T", expr))
	}
}

func (r *resolver) stmtList(stmts []ast.Stmt) {
	for _, s := range stmts {
		r.stmt(s)
	}
}

func (r *resolver) exprList(exprs []ast.Expr) {
	for _, x := range exprs {
		r.expr(x)
	}
}

func (r *resolver) ident(id *ast.Ident) {
	// Map this ident. If the name is already in scope, use that definition.
	// Otherwise check if it's an imported name.
	if obj := r.scope.LookupParent(id.Name); obj != nil {
		r.Idents[id] = obj
	} else if path := r.NameToPath[id.Name]; path != "" {
		r.Idents[id] = &Name{
			ImportPath: path,
		}
	}
}

func (r *resolver) define(id *ast.Ident, name *Name) {
	r.Idents[id] = name
	r.scope.Insert(id.Name, name)
}

func (r *resolver) openScope() {
	r.scope = newScope(r.scope)
}

func (r *resolver) closeScope() {
	r.scope = r.scope.Pop()
}

// scope maps names to information about them.
type scope struct {
	names  map[string]*Name
	parent *scope
}

func newScope(parent *scope) *scope {
	return &scope{
		names:  make(map[string]*Name),
		parent: parent,
	}
}

func (s *scope) Pop() *scope {
	return s.parent
}

func (s *scope) Insert(name string, r *Name) {
	if name != "_" {
		s.names[name] = r
	}
}

func (s *scope) Lookup(name string) *Name {
	return s.names[name]
}

func (s *scope) LookupParent(name string) *Name {
	if r := s.names[name]; r != nil {
		return r
	} else if s.parent != nil {
		return s.parent.LookupParent(name)
	}
	return nil
}
