package resource

import (
	"go/ast"
	"go/token"

	"encr.dev/v2/internal/pkginfo"
)

type Bind interface {
	Pos() token.Pos
	ResourceRef() ResourceOrPath
	Package() *pkginfo.Package
}

// A PkgDeclBind is a bind consisting of a package declaration.
type PkgDeclBind struct {
	Resource ResourceOrPath
	Pkg      *pkginfo.Package

	// BoundName is the package-level identifier the bind is declared with.
	BoundName *ast.Ident
}

func (b *PkgDeclBind) Pos() token.Pos              { return b.BoundName.Pos() }
func (b *PkgDeclBind) ResourceRef() ResourceOrPath { return b.Resource }
func (b *PkgDeclBind) Package() *pkginfo.Package   { return b.Pkg }

// QualifiedName returns the qualified name of the resource.
func (b *PkgDeclBind) QualifiedName() pkginfo.QualifiedName {
	return pkginfo.QualifiedName{
		PkgPath: b.Pkg.ImportPath,
		Name:    b.BoundName.Name,
	}
}

// ResourceOrPath is a reference to a particular resource,
// either referencing the resource object directly
// or through a path.
type ResourceOrPath struct {
	Resource Resource
	Path     Path
}

type Path []PathEntry

type PathEntry struct {
	Kind Kind
	Name string
}

func (p *Pass) AddBind(boundName *ast.Ident, resource Resource) {
	if boundName.Name == "_" {
		return
	}

	p.binds = append(p.binds, &PkgDeclBind{
		Resource:  ResourceOrPath{Resource: resource},
		Pkg:       p.Pkg,
		BoundName: boundName,
	})
}

func (p *Pass) AddPathBind(boundName *ast.Ident, path Path) {
	if len(path) == 0 {
		panic("AddPathBind: empty path")
	} else if boundName.Name == "_" {
		return
	}

	p.binds = append(p.binds, &PkgDeclBind{
		Resource:  ResourceOrPath{Path: path},
		Pkg:       p.Pkg,
		BoundName: boundName,
	})
}
