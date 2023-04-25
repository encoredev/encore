package resource

import (
	"go/ast"
	"go/token"

	"encr.dev/pkg/option"
	"encr.dev/v2/internals/pkginfo"
)

type Bind interface {
	Pos() token.Pos
	ResourceRef() ResourceOrPath
	Package() *pkginfo.Package
	DeclaredIn() option.Option[*pkginfo.File]

	// DescriptionForTest describes the bind for testing purposes.
	DescriptionForTest() string
}

// A PkgDeclBind is a bind consisting of a package declaration.
type PkgDeclBind struct {
	Resource ResourceOrPath
	File     *pkginfo.File

	// BoundName is the package-level identifier the bind is declared with.
	BoundName *ast.Ident
}

func (b *PkgDeclBind) Pos() token.Pos              { return b.BoundName.Pos() }
func (b *PkgDeclBind) ResourceRef() ResourceOrPath { return b.Resource }
func (b *PkgDeclBind) Package() *pkginfo.Package   { return b.File.Pkg }
func (b *PkgDeclBind) DescriptionForTest() string  { return b.QualifiedName().NaiveDisplayName() }
func (b *PkgDeclBind) DeclaredIn() option.Option[*pkginfo.File] {
	return option.Some(b.File)
}

// QualifiedName returns the qualified name of the resource.
func (b *PkgDeclBind) QualifiedName() pkginfo.QualifiedName {
	return pkginfo.QualifiedName{
		PkgPath: b.File.Pkg.ImportPath,
		Name:    b.BoundName.Name,
	}
}

// An AnonymousBind is similar to PkgDeclBind in that it's a package declaration,
// but unlike PkgDeclBind it's bound to an "_" identifier so that it has no name.
type AnonymousBind struct {
	Resource ResourceOrPath
	File     *pkginfo.File
}

func (b *AnonymousBind) Pos() token.Pos              { return b.File.Pkg.AST.Pos() }
func (b *AnonymousBind) ResourceRef() ResourceOrPath { return b.Resource }
func (b *AnonymousBind) Package() *pkginfo.Package   { return b.File.Pkg }
func (b *AnonymousBind) DescriptionForTest() string  { return "anonymous" }
func (b *AnonymousBind) DeclaredIn() option.Option[*pkginfo.File] {
	return option.Some(b.File)
}

// An ImplicitBind is a bind that implicitly binds to a package and its subpackages.
type ImplicitBind struct {
	Resource ResourceOrPath
	Pkg      *pkginfo.Package
}

func (b *ImplicitBind) Pos() token.Pos              { return b.Pkg.AST.Pos() }
func (b *ImplicitBind) ResourceRef() ResourceOrPath { return b.Resource }
func (b *ImplicitBind) Package() *pkginfo.Package   { return b.Pkg }
func (b *ImplicitBind) DescriptionForTest() string  { return "implicit" }
func (b *ImplicitBind) DeclaredIn() option.Option[*pkginfo.File] {
	return option.None[*pkginfo.File]()
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
