package usage

import (
	"fmt"
	"go/ast"

	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/resource"
)

// Usage describes an infrastructure usage being used.
type Usage interface {
	ResourceBind() resource.Bind
	ASTExpr() ast.Expr
	DeclaredIn() *pkginfo.File

	// DescriptionForTest describes the usage for testing purposes.
	DescriptionForTest() string
}

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

// Other describes any other resource usage.
type Other struct {
	File *pkginfo.File
	Bind resource.Bind
	Expr ast.Expr
}

func (o *Other) DeclaredIn() *pkginfo.File   { return o.File }
func (o *Other) ASTExpr() ast.Expr           { return o.Expr }
func (o *Other) ResourceBind() resource.Bind { return o.Bind }
func (o *Other) DescriptionForTest() string  { return "other" }

func Parse(pkgs []*pkginfo.Package, binds []resource.Bind) []Usage {
	p := &usageParser{
		bindsPerPkg: make(map[paths.Pkg][]resource.Bind, len(binds)),
		bindNames:   make(map[pkginfo.QualifiedName]resource.Bind, len(binds)),
	}
	for _, b := range binds {
		pkg := b.Package
		p.bindsPerPkg[pkg.ImportPath] = append(p.bindsPerPkg[pkg.ImportPath], b)
		p.bindNames[b.QualifiedName()] = b
	}

	var usages []Usage
	for _, pkg := range pkgs {
		usages = append(usages, p.scanUsage(pkg)...)
	}
	return usages
}

type usageParser struct {
	bindsPerPkg map[paths.Pkg][]resource.Bind
	bindNames   map[pkginfo.QualifiedName]resource.Bind
}

func (p *usageParser) scanUsage(pkg *pkginfo.Package) (usages []Usage) {
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
						if u := p.classifyUsage(f, bind, stack); u != nil {
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
	if pkg.ImportPath != bind.Package.ImportPath {
		return false
	}

	switch x := expr.(type) {
	case *ast.SelectorExpr:
		return x.Sel == bind.BoundName
	case *ast.Ident:
		return x == bind.BoundName
	}
	return false
}

func (p *usageParser) classifyUsage(file *pkginfo.File, bind resource.Bind, stack []ast.Node) Usage {
	idx := len(stack) - 1

	if idx >= 1 {
		if sel, ok := stack[idx-1].(*ast.SelectorExpr); ok {

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
	}

	// It's some other kind of usage. Find the largest enclosing expression.
	enclosing := stack[idx].(ast.Expr) // guaranteed to be an expr by the caller.
	for i := idx; i >= 0; i-- {
		if expr, ok := stack[i].(ast.Expr); ok {
			enclosing = expr
		}
	}

	return &Other{
		File: file,
		Bind: bind,
		Expr: enclosing,
	}
}
