package usage

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/option"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/resource"
)

type Expr interface {
	// Node allows use to use the expression in error messages
	// where the Pos/End is used to point directly at the resource
	// bind being used, rather than the overall expression.
	ast.Node

	ResourceBind() resource.Bind
	ASTExpr() ast.Expr
	DeclaredIn() *pkginfo.File

	// DescriptionForTest describes the expression for testing purposes.
	DescriptionForTest() string
}

// FuncCall describes a resource being called as a function.
type FuncCall struct {
	File *pkginfo.File
	Bind resource.Bind
	Call *ast.CallExpr
	Args []ast.Expr
}

func (f *FuncCall) DeclaredIn() *pkginfo.File   { return f.File }
func (f *FuncCall) ASTExpr() ast.Expr           { return f.Call }
func (f *FuncCall) ResourceBind() resource.Bind { return f.Bind }
func (f *FuncCall) DescriptionForTest() string  { return "called" }
func (f *FuncCall) Pos() token.Pos              { return f.Call.Fun.Pos() }
func (f *FuncCall) End() token.Pos              { return f.Call.Fun.End() }

// MethodCall describes a resource usage via a method call.
type MethodCall struct {
	File   *pkginfo.File
	Bind   resource.Bind
	Call   *ast.CallExpr
	Method string
	Args   []ast.Expr
}

func (m *MethodCall) DeclaredIn() *pkginfo.File   { return m.File }
func (m *MethodCall) ASTExpr() ast.Expr           { return m.Call }
func (m *MethodCall) ResourceBind() resource.Bind { return m.Bind }
func (m *MethodCall) DescriptionForTest() string  { return fmt.Sprintf("call %s", m.Method) }
func (m *MethodCall) Pos() token.Pos              { return m.Call.Fun.Pos() }
func (m *MethodCall) End() token.Pos              { return m.Call.Fun.End() }

// FieldAccess describes a resource usage via a field access.
type FieldAccess struct {
	File  *pkginfo.File
	Bind  resource.Bind
	Expr  *ast.SelectorExpr
	Field string
}

func (f *FieldAccess) DeclaredIn() *pkginfo.File   { return f.File }
func (f *FieldAccess) ASTExpr() ast.Expr           { return f.Expr }
func (f *FieldAccess) ResourceBind() resource.Bind { return f.Bind }
func (f *FieldAccess) DescriptionForTest() string  { return fmt.Sprintf("field %s", f.Field) }
func (f *FieldAccess) Pos() token.Pos              { return f.Expr.Sel.Pos() }
func (f *FieldAccess) End() token.Pos              { return f.Expr.Sel.End() }

// FuncArg describes a resource being used as a function argument.
type FuncArg struct {
	File *pkginfo.File
	Bind resource.Bind
	Call *ast.CallExpr

	// ArgIdx is the function argument index that represents
	// the resource bind, starting at 0.
	ArgIdx int

	// PkgFunc is the package-level function that's being called.
	// It's None if the function is not a package-level function.
	PkgFunc option.Option[pkginfo.QualifiedName]
}

func (f *FuncArg) DeclaredIn() *pkginfo.File   { return f.File }
func (f *FuncArg) ASTExpr() ast.Expr           { return f.Call }
func (f *FuncArg) ResourceBind() resource.Bind { return f.Bind }
func (f *FuncArg) DescriptionForTest() string {
	if fn, ok := f.PkgFunc.Get(); ok {
		return fmt.Sprintf("fn %s arg %d", fn.NaiveDisplayName(), f.ArgIdx)
	}
	return fmt.Sprintf("arg %d", f.ArgIdx)
}
func (f *FuncArg) Pos() token.Pos { return f.Call.Args[f.ArgIdx].Pos() }
func (f *FuncArg) End() token.Pos { return f.Call.Args[f.ArgIdx].End() }

// Other describes any other resource usage.
type Other struct {
	File    *pkginfo.File
	Bind    resource.Bind
	Expr    ast.Expr
	BindRef ast.Node
}

func (o *Other) DeclaredIn() *pkginfo.File   { return o.File }
func (o *Other) ASTExpr() ast.Expr           { return o.Expr }
func (o *Other) ResourceBind() resource.Bind { return o.Bind }
func (o *Other) DescriptionForTest() string  { return "other" }
func (o *Other) Pos() token.Pos              { return o.BindRef.Pos() }
func (o *Other) End() token.Pos              { return o.BindRef.End() }

func ParseExprs(errs *perr.List, pkgs []*pkginfo.Package, binds []resource.Bind) []Expr {
	p := &usageParser{
		bindsPerPkg: make(map[paths.Pkg][]resource.Bind, len(binds)),
		bindNames:   make(map[pkginfo.QualifiedName]resource.Bind, len(binds)),
	}
	for _, b := range binds {
		pkg := b.Package()
		p.bindsPerPkg[pkg.ImportPath] = append(p.bindsPerPkg[pkg.ImportPath], b)

		if pkgDecl, ok := b.(*resource.PkgDeclBind); ok {
			p.bindNames[pkgDecl.QualifiedName()] = b
		}
	}

	var usages []Expr
	for _, pkg := range pkgs {
		usages = append(usages, p.scanUsage(pkg)...)
	}

	// Compute implicit usage of sqldb resources.
	usages = append(usages, computeImplicitSQLDBUsage(errs, pkgs, binds)...)

	return usages
}

type usageParser struct {
	schema      *schema.Parser
	bindsPerPkg map[paths.Pkg][]resource.Bind
	bindNames   map[pkginfo.QualifiedName]resource.Bind
}

func (p *usageParser) scanUsage(pkg *pkginfo.Package) (usages []Expr) {
	external, internal, files := p.bindsToScanFor(pkg)

	haveExternal := len(external) > 0
	haveInternal := len(internal) > 0

	// Compute types to scan for.
	var types []ast.Node
	if haveExternal {
		types = append(types, (*ast.SelectorExpr)(nil))
	}
	if haveInternal {
		types = append(types, (*ast.Ident)(nil))
	}

	for _, f := range files {
		inspector := f.ASTInspector()
		names := f.Names()
		inspector.WithStack(types, func(n ast.Node, push bool, stack []ast.Node) bool {
			if !push {
				return true
			}

			// If we're scanning for both *ast.SelectorExpr and *ast.Ident,
			// we will first scan the *ast.SelectorExpr and then recurse and then scan the *ast.Ident.
			// To avoid having to deal with this case, detect this case and ignore the *ast.Ident
			// the second time around.
			if haveExternal && haveInternal {
				if id, ok := n.(*ast.Ident); ok {
					if sel, ok := stack[len(stack)-2].(*ast.SelectorExpr); ok && sel.Sel == id {
						return true
					}
				}
			}

			expr := n.(ast.Expr) // guaranteed since the types we scan for are all expressions
			if qn, ok := names.ResolvePkgLevelRef(expr); ok {
				if bind, ok := p.bindNames[qn]; ok {
					// Make sure this is not the actual bind definition, to avoid reporting spurious usages.
					if !p.isBind(pkg, expr, bind) {
						if u := p.classifyExpr(f, bind, stack); u != nil {
							usages = append(usages, u)
						}
					}
				}
			}
			return true
		})
	}

	return usages
}

// bindsToScanFor returns the binds to scan for in a given package,
// and which files to scan.
// The 'external' binds are those that are imported from other packages,
// and 'internal' binds are those that are defined in the same package.
func (p *usageParser) bindsToScanFor(pkg *pkginfo.Package) (external, internal []resource.Bind, files []*pkginfo.File) {
	internal = p.bindsPerPkg[pkg.ImportPath]

	if len(internal) > 0 {
		// If we have any internal binds we need to scan all files,
		// since we can't rely on the file-level imports to tell us
		// which files to scan.
		files = pkg.Files
	}

	for imp := range pkg.Imports {
		external = append(external, p.bindsPerPkg[imp]...)
	}

	if len(external) > 0 && len(internal) == 0 {
		// If we have external binds but no internal binds,
		// figure out which files to parse precisely.
	FileLoop:
		for _, f := range pkg.Files {
			for imp := range f.Imports {
				if len(p.bindsPerPkg[imp]) > 0 {
					files = append(files, f)
					continue FileLoop
				}
			}
		}
	}

	return
}

func (p *usageParser) isBind(pkg *pkginfo.Package, expr ast.Expr, bind resource.Bind) bool {
	if pkg.ImportPath != bind.Package().ImportPath {
		return false
	}

	// If the bind isn't a package decl it can't be this bind.
	pkgDecl, ok := bind.(*resource.PkgDeclBind)
	if !ok {
		return false
	}

	switch x := expr.(type) {
	case *ast.SelectorExpr:
		return x.Sel == pkgDecl.BoundName
	case *ast.Ident:
		return x == pkgDecl.BoundName
	}
	return false
}

func (p *usageParser) classifyExpr(file *pkginfo.File, bind resource.Bind, stack []ast.Node) Expr {
	idx := len(stack) - 1

	if idx >= 1 {
		if sel, ok := stack[idx-1].(*ast.SelectorExpr); ok {
			// bind.SomeField or bind.SomeMethod()

			// Check if this is a method call
			if idx >= 2 {
				if call, ok := stack[idx-2].(*ast.CallExpr); ok {
					return &MethodCall{
						File:   file,
						Bind:   bind,
						Call:   call,
						Method: sel.Sel.Name,
						Args:   call.Args,
					}
				}
			}

			// Otherwise it's a field access
			return &FieldAccess{
				File:  file,
				Bind:  bind,
				Expr:  sel,
				Field: sel.Sel.Name,
			}
		}

		// Is this bind being referenced in a function call argument?
		if call, ok := stack[idx-1].(*ast.CallExpr); ok {

			if call.Fun == stack[idx] {
				return &FuncCall{
					File: file,
					Bind: bind,
					Call: call,
					Args: call.Args,
				}
			}

			// Find which argument this is.
			if argIdx := slices.Index(call.Args, stack[idx].(ast.Expr)); argIdx >= 0 {
				pkgFunc := option.CommaOk(file.Names().ResolvePkgLevelRef(call.Fun))
				return &FuncArg{
					File:    file,
					Bind:    bind,
					Call:    call,
					PkgFunc: pkgFunc,
					ArgIdx:  argIdx,
				}
			}
		}
	}

	// It's some other kind of usage. Find the largest enclosing expression.
	enclosing := stack[idx].(ast.Expr) // guaranteed to be an expr by the caller.
	for i := idx; i >= 0; i-- {
		if expr, ok := stack[i].(ast.Expr); ok {
			enclosing = expr
		}
	}

	return &Other{
		File:    file,
		Bind:    bind,
		Expr:    enclosing,
		BindRef: stack[idx],
	}
}
