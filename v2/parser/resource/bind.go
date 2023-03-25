package resource

import (
	"go/ast"
	"go/token"

	"encr.dev/v2/internals/pkginfo"
)

type Bind interface {
	Pos() token.Pos
	ResourceRef() ResourceOrPath
	Package() *pkginfo.Package

	// DescriptionForTest describes the bind for testing purposes.
	DescriptionForTest() string
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
func (b *PkgDeclBind) DescriptionForTest() string  { return b.QualifiedName().NaiveDisplayName() }

// QualifiedName returns the qualified name of the resource.
func (b *PkgDeclBind) QualifiedName() pkginfo.QualifiedName {
	return pkginfo.QualifiedName{
		PkgPath: b.Pkg.ImportPath,
		Name:    b.BoundName.Name,
	}
}

// An AnonymousBind is similar to PkgDeclBind in that it's a package declaration,
// but unlike PkgDeclBind it's bound to an "_" identifier so that it has no name.
type AnonymousBind struct {
	Resource ResourceOrPath
	Pkg      *pkginfo.Package
}

func (b *AnonymousBind) Pos() token.Pos              { return b.Pkg.AST.Pos() }
func (b *AnonymousBind) ResourceRef() ResourceOrPath { return b.Resource }
func (b *AnonymousBind) Package() *pkginfo.Package   { return b.Pkg }
func (b *AnonymousBind) DescriptionForTest() string  { return "anonymous" }

// An ImplicitBind is a bind that implicitly binds to a package and its subpackages.
type ImplicitBind struct {
	Resource ResourceOrPath
	Pkg      *pkginfo.Package
}

func (b *ImplicitBind) Pos() token.Pos              { return b.Pkg.AST.Pos() }
func (b *ImplicitBind) ResourceRef() ResourceOrPath { return b.Resource }
func (b *ImplicitBind) Package() *pkginfo.Package   { return b.Pkg }
func (b *ImplicitBind) DescriptionForTest() string  { return "implicit" }

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
