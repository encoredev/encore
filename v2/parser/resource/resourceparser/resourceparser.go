package resourceparser

import (
	"go/ast"

	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/protoparse"
	"encr.dev/v2/internals/schema"
	"encr.dev/v2/parser/resource"
)

type Parser struct {
	Name string

	// InterestingImports are the imports paths that the parser is interested in.
	// If a package imports any one of them, the Run method is invoked.
	InterestingImports []paths.Pkg

	// InterestingSubdirs are the subdirectories of a package that a parser is interested in.
	// If a package has any one of these subdirectories, the Run method is invoked.
	// Its purpose is to support our current way of defining databases via a "migrations" dir.
	InterestingSubdirs []string

	Run func(*Pass)
}

// RunAlways is a value for InterestingImports to indicate to always run the parser.
var RunAlways = []paths.Pkg{"*"}

type Pass struct {
	*parsectx.Context
	SchemaParser *schema.Parser
	ProtoParser  *protoparse.Parser

	Pkg *pkginfo.Package

	resources []resource.Resource
	binds     []resource.Bind
}

func (p *Pass) RegisterResource(resource resource.Resource) {
	p.resources = append(p.resources, resource)
}

func (p *Pass) Resources() []resource.Resource {
	return p.resources
}

func (p *Pass) Binds() []resource.Bind {
	return p.binds
}

func (p *Pass) AddBind(file *pkginfo.File, boundName option.Option[*ast.Ident], res resource.Resource) {
	if id, ok := boundName.Get(); ok {
		p.AddNamedBind(file, id, res)
	} else {
		p.AddAnonymousBind(file, res)
	}
}

func (p *Pass) AddNamedBind(file *pkginfo.File, boundName *ast.Ident, res resource.Resource) {
	if boundName.Name == "_" {
		p.AddAnonymousBind(file, res)
	} else {
		p.binds = append(p.binds, &resource.PkgDeclBind{
			Resource:  resource.ResourceOrPath{Resource: res},
			File:      file,
			BoundName: boundName,
		})
	}
}

func (p *Pass) AddAnonymousBind(file *pkginfo.File, res resource.Resource) {
	p.binds = append(p.binds, &resource.AnonymousBind{
		Resource: resource.ResourceOrPath{Resource: res},
		File:     file,
	})
}

func (p *Pass) AddPathBind(file *pkginfo.File, boundName *ast.Ident, path resource.Path) {
	if len(path) == 0 {
		panic("AddPathBind: empty path")
	}

	if boundName.Name == "_" {
		p.binds = append(p.binds, &resource.AnonymousBind{
			Resource: resource.ResourceOrPath{Path: path},
			File:     file,
		})
	} else {
		p.binds = append(p.binds, &resource.PkgDeclBind{
			Resource:  resource.ResourceOrPath{Path: path},
			File:      file,
			BoundName: boundName,
		})
	}
}

func (p *Pass) AddImplicitBind(res resource.Resource) {
	p.binds = append(p.binds, &resource.ImplicitBind{
		Resource: resource.ResourceOrPath{Resource: res},
		Pkg:      p.Pkg,
	})
}
