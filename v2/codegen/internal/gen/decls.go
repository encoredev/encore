package gen

import (
	"io"
	"strings"

	"github.com/dave/jennifer/jen"

	"encr.dev/v2/internal/pkginfo"
)

func newFile(pkg *pkginfo.Package, suffix string) *File {
	jenFile := jen.NewFilePathName(pkg.ImportPath.String(), pkg.Name)
	return &File{
		Pkg:    pkg,
		Jen:    jenFile,
		suffix: suffix,
	}
}

// File represents a generated file for a specific package.
type File struct {
	Pkg    *pkginfo.Package // the package the file belongs to
	Jen    *jen.File        // the jen file we're generating
	suffix string           // the file name suffix. "metrics" for "encore_internal__metrics.go".
	decls  []Decl
}

// Name returns the computed file name.
func (f *File) Name() string {
	return "encore_internal__" + f.suffix + ".go"
}

// Render renders the file to the given writer.
func (f *File) Render(w io.Writer) error {
	for i, d := range f.decls {
		if i > 0 {
			f.Jen.Line()
		}
		f.Jen.Add(d.Code())
	}
	return f.Jen.Render(w)
}

type Decl interface {
	Name() string
	Qual() *jen.Statement
	Code() *jen.Statement
}

func (f *File) FuncDecl(nameParts ...string) *FuncDecl {
	if len(nameParts) == 0 {
		panic("gen.VarDecl: empty nameParts")
	}
	d := &FuncDecl{
		File:      f,
		nameParts: nameParts,
	}
	f.decls = append(f.decls, d)
	return d
}

func (f *File) VarDecl(nameParts ...string) *VarDecl {
	if len(nameParts) == 0 {
		panic("gen.VarDecl: empty nameParts")
	}
	d := &VarDecl{
		File:      f,
		nameParts: nameParts,
	}
	f.decls = append(f.decls, d)
	return d
}

// FuncDecl represents a generated declaration.
type FuncDecl struct {
	File *File // file the declaration belongs to.

	// nameParts are the suffix parts of the generated name.
	// For example, if the parts are ["foo", "bar"] and the
	// file suffix is "metrics", the generated declaration name
	// is "EncoreInternal_metrics_foo_bar".
	nameParts []string

	// typeParams are the type parameters of the generated function.
	typeParams []jen.Code

	// params are the parameters of the generated function.
	params []jen.Code

	// results are the results of the generated function.
	results []jen.Code

	// body is the body of the generated function.
	body *jen.Statement
}

// Name returns the package-level name of the declaration.
func (d *FuncDecl) Name() string {
	return "EncoreInternal_" + d.File.suffix + "_" + strings.Join(d.nameParts, "_")
}

// Qual returns the qualified name of the declaration.
func (d *FuncDecl) Qual() *jen.Statement {
	return jen.Qual(d.File.Pkg.ImportPath.String(), d.Name())
}

// Code returns the generated code.
func (d *FuncDecl) Code() *jen.Statement {
	s := jen.Func().Id(d.Name())
	if len(d.typeParams) > 0 {
		s = s.Types(d.typeParams...)
	}
	if len(d.params) > 0 {
		s = s.Params(d.params...)
	}
	if len(d.results) > 0 {
		s = s.Params(d.results...)
	}
	if d.body != nil {
		s = s.Block(d.body)
	} else {
		s = s.Block()
	}
	return s
}

// TypeParams appends to the type parameters of the generated function.
func (d *FuncDecl) TypeParams(params ...jen.Code) *FuncDecl {
	d.typeParams = append(d.typeParams, params...)
	return d
}

// Params appends to the parameters of the generated function.
func (d *FuncDecl) Params(params ...jen.Code) *FuncDecl {
	d.params = append(d.params, params...)
	return d
}

// Results appends to the results of the generated function.
func (d *FuncDecl) Results(results ...jen.Code) *FuncDecl {
	d.results = append(d.results, results...)
	return d
}

// Body sets the body of the generated function.
func (d *FuncDecl) Body(code ...jen.Code) *FuncDecl {
	d.body = jen.Block(code...)
	return d
}

// BodyFunc computes the body of the generated function.
func (d *FuncDecl) BodyFunc(fn func(g *jen.Group)) *FuncDecl {
	d.body = jen.BlockFunc(fn)
	return d
}

type VarDecl struct {
	File *File // file the declaration belongs to.

	// nameParts are the suffix parts of the generated name.
	// For example, if the parts are ["foo", "bar"] and the
	// file suffix is "metrics", the generated declaration name
	// is "EncoreInternal_metrics_foo_bar".
	nameParts []string

	value *jen.Statement
}

func (d *VarDecl) Value(code ...jen.Code) *VarDecl {
	d.value = jen.Add(code...)
	return d
}

func (d *VarDecl) Code() *jen.Statement {
	return jen.Var().Id(d.Name()).Op("=").Add(d.value)
}

func (d *VarDecl) Name() string {
	return "EncoreInternal_" + d.File.suffix + "_" + strings.Join(d.nameParts, "_")
}

func (d *VarDecl) Qual() *jen.Statement {
	return jen.Qual(d.File.Pkg.ImportPath.String(), d.Name())
}
