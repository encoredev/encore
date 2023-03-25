package schema

import (
	"go/ast"

	"golang.org/x/exp/slices"

	"encr.dev/internal/paths"
	"encr.dev/pkg/option"
	"encr.dev/v2/internals/pkginfo"
)

// Decl is the common interface for different kinds of declarations.
type Decl interface {
	Kind() DeclKind

	DeclaredIn() *pkginfo.File
	// PkgName reports the name if this is a package-level declaration.
	// Otherwise it reports None.
	PkgName() option.Option[string]

	// ASTNode returns the AST node that this declaration represents.
	// It's a *ast.FuncDecl or *ast.TypeSpec.
	ASTNode() ast.Node
	// String returns the shorthand name for this declaration,
	// in the form "pkgname.DeclName".
	String() string
	// TypeParameters are the type parameters on this declaration.
	TypeParameters() []DeclTypeParam
}

// DeclKind represents different kinds of declarations.
type DeclKind int

const (
	// DeclType represents a type declaration.
	DeclType DeclKind = iota
	// DeclFunc represents a func declaration.
	DeclFunc
)

// TypeDecl represents a type declaration.
type TypeDecl struct {
	// AST is the AST node that this declaration represents.
	AST *ast.TypeSpec

	Info *pkginfo.PkgDeclInfo // the underlying declaration info
	File *pkginfo.File        // the file declaring the type
	Name string               // name of the type declaration
	Type Type                 // the declaration's underlying type

	// TypeParams are any type parameters on this declaration.
	// (note: instantiated types used within this declaration would not be captured here)
	TypeParams []DeclTypeParam
}

func (d *TypeDecl) Clone() *TypeDecl {
	return &TypeDecl{
		AST:        d.AST,
		Info:       d.Info,
		File:       d.File,
		Name:       d.Name,
		Type:       d.Type,
		TypeParams: slices.Clone(d.TypeParams),
	}
}

// DeclTypeParam represents a type parameter on a declaration.
// For example A in "type Foo[A any] struct { ... }"
type DeclTypeParam struct {
	// AST is the AST node that this type param represents.
	// Note that multiple fields may share the same *ast.Field node,
	// in cases with multiple names, like "type Foo[A, B any]".
	AST *ast.Field

	Name string // the identifier given to the type parameter.
}

// A FuncDecl represents a function declaration.
type FuncDecl struct {
	AST *ast.FuncDecl

	File *pkginfo.File            // the file declaring the type
	Name string                   // the name of the function
	Recv option.Option[*Receiver] // none if not a method
	Type FuncType                 // signature

	// TypeParams are any type parameters on this declaration.
	// (note: instantiated types used within this declaration would not be captured here)
	TypeParams []DeclTypeParam
}

// A Receiver represents a method receiver.
// It describes the name and type of the receiver.
type Receiver struct {
	AST *ast.FieldList
	// Name is the name of the receiver (e.g. "a" in "func (a *Foo) Bar()").
	// It's None if the receiver is unnamed (e.g. "func (*Foo) Bar()").
	Name option.Option[string]

	// Type is the type of the receiver.
	// It's either a NamedType or a NamedType wrapped in a PointerType.
	Type Type

	// Decl is the underlying type declaration the receiver points to.
	Decl *TypeDecl
}

func (*TypeDecl) Kind() DeclKind                    { return DeclType }
func (d *TypeDecl) DeclaredIn() *pkginfo.File       { return d.File }
func (d *TypeDecl) ASTNode() ast.Node               { return d.AST }
func (d *TypeDecl) String() string                  { return d.File.Pkg.Name + "." + d.Name }
func (d *TypeDecl) TypeParameters() []DeclTypeParam { return d.TypeParams }
func (d *TypeDecl) PkgName() option.Option[string]  { return option.Some(d.Name) }

func (*FuncDecl) Kind() DeclKind                    { return DeclFunc }
func (d *FuncDecl) DeclaredIn() *pkginfo.File       { return d.File }
func (d *FuncDecl) ASTNode() ast.Node               { return d.AST }
func (d *FuncDecl) String() string                  { return d.File.Pkg.Name + "." + d.Name }
func (d *FuncDecl) TypeParameters() []DeclTypeParam { return d.TypeParams }
func (d *FuncDecl) PkgName() option.Option[string] {
	if d.Recv.Empty() {
		return option.Some(d.Name)
	}
	return option.None[string]()
}

// ParseTypeDecl parses the type from a package declaration.
// It errors if the declaration is not a type.
func (p *Parser) ParseTypeDecl(d *pkginfo.PkgDeclInfo) *TypeDecl {
	pkg := d.File.Pkg

	// Have we already parsed this?
	key := declKey{pkg: pkg.ImportPath, name: d.Name}
	p.declsMu.Lock()
	cached, ok := p.decls[key]
	p.declsMu.Unlock()

	if ok {
		if td, ok := cached.(*TypeDecl); ok {
			return td
		} else {
			p.c.Errs.Fatalf(d.Spec.Pos(), "decl %s is not a TypeDecl", d.Name)
		}
	}

	// We haven't parsed this yet; do so now.
	// Allocate a decl immediately so that we can properly handle
	// recursive types by short-circuiting above the second time we get here.
	spec, ok := d.Spec.(*ast.TypeSpec)
	if !ok {
		p.c.Errs.Fatal(d.Spec.Pos(), "unable to get TypeSpec from PkgDecl spec")
	}

	decl := &TypeDecl{
		AST:        spec,
		Name:       d.Name,
		File:       d.File,
		Info:       d,
		TypeParams: nil,
		// Type is set below
	}
	p.declsMu.Lock()
	p.decls[key] = decl
	p.declsMu.Unlock()

	// If this is a parameterized declaration, get the type parameters
	var typeParamsInScope map[string]int
	typeParamsInScope, decl.TypeParams = computeDeclTypeParams(spec.TypeParams)

	r := p.newTypeResolver(decl, typeParamsInScope)
	decl.Type = r.parseType(d.File, spec.Type)

	return decl
}

// ParseFuncDecl parses the func from a package declaration.
// It errors if the type is not a func declaration.
func (p *Parser) ParseFuncDecl(file *pkginfo.File, fd *ast.FuncDecl) (*FuncDecl, bool) {
	// Have we already parsed this?
	key := declKey{pkg: file.Pkg.ImportPath, name: fd.Name.Name}

	// Is there a receiver? If so we need to add that to the cache key.
	var recv option.Option[*Receiver]
	if fd.Recv != nil {
		r, ok := p.parseRecv(file, fd.Recv)
		if !ok {
			return nil, false
		}
		key.recvName = r.Decl.Name
		recv = option.Some(r)
	}

	p.declsMu.Lock()
	cached, ok := p.decls[key]
	p.declsMu.Unlock()

	if ok {
		if fd, ok := cached.(*FuncDecl); ok {
			return fd, true
		} else {
			p.c.Errs.Add(errDeclIsntFunction(fd.Name).AtGoNode(fd.AST))
			return nil, false
		}
	}

	// We haven't parsed this yet; do so now.
	// Allocate a decl immediately so that we can properly handle
	// recursive types by short-circuiting above the second time we get here.

	decl := &FuncDecl{
		AST:  fd,
		File: file,
		Name: fd.Name.Name,
		Recv: recv,
		// Type, and TypeParams are set below
	}
	p.declsMu.Lock()
	p.decls[key] = decl
	p.declsMu.Unlock()

	// If this is a parameterized declaration, get the type parameters.
	// If the function is a method, get the type parameters from the receiver's type declaration.
	// Otherwise, use the ones on the function, if any.
	var typeParamsInScope map[string]int
	if recv.Present() {
		// Type params on the receiver do not become type params on the func declaration,
		// so we use "_" to ignore that value, unlike in the "else" case below.
		typeParamsInScope, _ = computeDeclTypeParams(recv.MustGet().Decl.AST.TypeParams)
	} else {
		typeParamsInScope, decl.TypeParams = computeDeclTypeParams(fd.Type.TypeParams)
	}

	// Resolve the function type.
	r := p.newTypeResolver(decl, typeParamsInScope)
	decl.Type = r.parseFuncType(file, fd.Type)
	return decl, true
}

// computeDeclTypeParams computes the type parameter placeholders for a declaration.
// For example, given the "[A, B any]" in the following declaration:
//
//	type Foo[A, B any] struct { ... }
//
// it returns:
//
//	nameMap = {"A": 0, "B": 1}
//	params = [{Name: "A"}, {Name: "B"}]
func computeDeclTypeParams(typeParams *ast.FieldList) (nameMap map[string]int, params []DeclTypeParam) {
	if typeParams == nil {
		return nil, nil
	}
	numParams := typeParams.NumFields()
	params = make([]DeclTypeParam, 0, numParams)
	nameMap = make(map[string]int, numParams)

	paramIdx := 0
	for _, typeParam := range typeParams.List {
		for _, name := range typeParam.Names {
			params = append(params, DeclTypeParam{
				AST:  typeParam,
				Name: name.Name,
			})
			nameMap[name.Name] = paramIdx
			paramIdx++
		}
	}
	return nameMap, params
}

// declKey is a unique key for the given declaration.
type declKey struct {
	pkg  paths.Pkg
	name string

	// recvName specifies the name of the receiver for method declarations.
	recvName string
}
