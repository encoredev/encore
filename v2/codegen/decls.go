package codegen

import (
	"fmt"
	"io"
	"strings"

	"github.com/dave/jennifer/jen"
	"golang.org/x/exp/slices"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
)

var importNames = map[string]string{
	"github.com/felixge/httpsnoop":        "httpsnoop",
	"github.com/json-iterator/go":         "jsoniter",
	"github.com/julienschmidt/httprouter": "httprouter",

	"encore.dev/appruntime/api":         "__api",
	"encore.dev/appruntime/app":         "__app",
	"encore.dev/appruntime/app/appinit": "__appinit",
	"encore.dev/appruntime/config":      "__config",
	"encore.dev/appruntime/etype":       "__etype",
	"encore.dev/appruntime/model":       "__model",
	"encore.dev/appruntime/serde":       "__serde",
	"encore.dev/appruntime/service":     "__service",
	"encore.dev/beta/errs":              "errs",
	"encore.dev/storage/sqldb":          "sqldb",
	"encore.dev/types/uuid":             "uuid",
}

func newFile(pkg *pkginfo.Package, baseName, shortName string) *File {
	return newFileForPath(pkg.ImportPath, pkg.Name, pkg.FSPath, baseName, shortName)
}

func newFileForPath(pkgPath paths.Pkg, pkgName string, pkgDir paths.FS, baseName, shortName string) *File {
	jenFile := jen.NewFilePathName(pkgPath.String(), pkgName)

	// Ensure the runtime is initialized before all generated code.
	jenFile.Anon("encore.dev/appruntime/app/appinit")

	for pkgPath, alias := range importNames {
		jenFile.ImportAlias(pkgPath, alias)
	}

	return &File{
		Jen:       jenFile,
		dir:       pkgDir,
		pkgPath:   pkgPath,
		baseName:  baseName,
		shortName: shortName,
	}
}

// File represents a generated file for a specific package.
type File struct {
	Jen *jen.File // the jen file we're generating

	// dir is the filesystem directory where the file should exist
	// within the application source tree. It need not match
	// any existing physical directory in the case of overlays.
	dir paths.FS

	pkgPath  paths.Pkg // the package the file belongs to
	baseName string    // the file's base name
	// shortName is the short version of the base name, for generated files.
	// For example if the base name is "encore_internal__metrics.go",
	// shortName is "metrics".
	shortName string

	decls []any // ordered list of Decl or jen.Code
}

// ImportAnon adds an anonymous ("_"-prefixed) import of the given packages.
func (f *File) ImportAnon(pkgs ...paths.Pkg) {
	f.Jen.Anon(fns.Map(pkgs, func(pkg paths.Pkg) string {
		return pkg.String()
	})...)
}

// name returns the computed file name.
func (f *File) name() string {
	return f.baseName
}

// Add adds a declaration to the file.
func (f *File) Add(code jen.Code) {
	if code != nil {
		f.decls = append(f.decls, code)
	}
}

// Render renders the file to the given writer.
func (f *File) Render(w io.Writer) error {
	for i, d := range f.decls {
		if i > 0 {
			f.Jen.Line()
		}

		switch d := d.(type) {
		case Decl:
			f.Jen.Add(d.Code())
		case jen.Code:
			f.Jen.Add(d)
		default:
			panic(fmt.Sprintf("internal error: unknown decl type: %T", d))
		}
	}
	return f.Jen.Render(w)
}

type Decl interface {
	Name() string
	Qual() *jen.Statement
	Code() *jen.Statement
}

func (f *File) HasDecl(nameParts ...string) bool {
	for _, decl := range f.decls {
		switch decl := decl.(type) {
		case *FuncDecl:
			if slices.Equal(decl.nameParts, nameParts) {
				return true
			}
		case *VarDecl:
			if slices.Equal(decl.nameParts, nameParts) {
				return true
			}
		}
	}
	return false
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
	// file shortName is "metrics", the generated declaration name
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
	return "EncoreInternal_" + d.File.shortName + "_" + strings.Join(d.nameParts, "_")
}

// Qual returns the qualified name of the declaration.
func (d *FuncDecl) Qual() *jen.Statement {
	return jen.Qual(d.File.pkgPath.String(), d.Name())
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
		s = s.Add(d.body)
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
	// file shortName is "metrics", the generated declaration name
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
	return "EncoreInternal_" + d.File.shortName + "_" + strings.Join(d.nameParts, "_")
}

func (d *VarDecl) Qual() *jen.Statement {
	return jen.Qual(d.File.pkgPath.String(), d.Name())
}
