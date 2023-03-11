package infra

import (
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/infra/resource"
)

func NewParser(c *parsectx.Context, schema *schema.Parser) *Parser {
	return &Parser{
		c:      c,
		schema: schema,
		reg:    newParserRegistry(allParsers),
	}
}

type Parser struct {
	c      *parsectx.Context
	schema *schema.Parser
	reg    *parserRegistry
}

// Parse parses all the infra resources defined in the given package.
func (p *Parser) Parse(pkg *pkginfo.Package) ([]resource.Resource, []resource.Bind) {
	interested := p.reg.InterestedParsers(pkg)
	if len(interested) == 0 {
		return nil, nil
	}

	pass := &resource.Pass{
		Context:      p.c,
		SchemaParser: p.schema,
		Pkg:          pkg,
	}
	for _, p := range interested {
		p.Run(pass)
	}
	return pass.Resources(), pass.Binds()
}

// ParseMulti is a helper function to parse a list of packages.
// It simply calls Parse on each package and combines the results.
func (p *Parser) ParseMulti(pkgs []*pkginfo.Package) ([]resource.Resource, []resource.Bind) {
	var allResources []resource.Resource
	var allBinds []resource.Bind
	for _, pkg := range pkgs {
		resources, binds := p.Parse(pkg)
		allResources = append(allResources, resources...)
		allBinds = append(allBinds, binds...)
	}
	return allResources, allBinds
}

// ComputeResult computes the application-wide result of parsing all infrastructure
// and validating it.
//
// Note: in the future this should operate on metadata and not the in-memory infra resources,
// to better work with an application split across multiple repositories.
func (p *Parser) ComputeResult(all []resource.Resource) *ParseResult {
	return &ParseResult{
		resources: all,
	}
}

// ParseResult is the combined, validated result of parsing all packages in a project.
type ParseResult struct {
	resources []resource.Resource
}

func (r *ParseResult) AllResources() []resource.Resource {
	return r.resources
}
