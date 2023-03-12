package parser

import (
	"sync"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/scan"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/infra"
	"encr.dev/v2/parser/infra/resource"
	"encr.dev/v2/parser/infra/usage"
)

func NewParser(c *parsectx.Context) *Parser {
	loader := pkginfo.New(c)
	schemaParser := schema.NewParser(c, loader)
	apiParser := apis.NewParser(c, schemaParser)
	infraParser := infra.NewParser(c, schemaParser)
	return &Parser{
		c:           c,
		loader:      loader,
		apiParser:   apiParser,
		infraParser: infraParser,
	}
}

type Parser struct {
	c           *parsectx.Context
	loader      *pkginfo.Loader
	apiParser   *apis.Parser
	infraParser *infra.Parser
}

func (p *Parser) MainModule() *pkginfo.Module {
	return p.loader.MainModule()
}

type Result struct {
	// Packages are the packages that are contained within
	// the application. It does not list packages that have been
	// parsed but belong to dependencies.
	Packages []*pkginfo.Package

	// APIs is a list of [apis.ParseResult] by package that uses the Encore API Framework
	// that was found during the parse. If a package does not use the API Framework, it won't
	// be included in the list.
	APIs []*apis.ParseResult

	// Infra describes the parsed infrastructure.
	Infra *infra.Desc
}

// Parse parses the given application for uses of the Encore API Framework
// and the Encore infrastructure SDK.
func (p *Parser) Parse() Result {
	var (
		mu           sync.Mutex
		appPkgs      []*pkginfo.Package
		allResources []resource.Resource
		allBinds     []resource.Bind
		apiResults   []*apis.ParseResult
	)

	scan.ProcessModule(p.c.Errs, p.loader, p.c.MainModuleDir, func(pkg *pkginfo.Package) {
		apiRes := p.apiParser.Parse(pkg)
		resources, binds := p.infraParser.Parse(pkg)

		mu.Lock()
		appPkgs = append(appPkgs, pkg)
		allResources = append(allResources, resources...)
		allBinds = append(allBinds, binds...)
		if apiRes != nil {
			apiResults = append(apiResults, apiRes)
		}
		mu.Unlock()
	})

	infraUsage := usage.Parse(p.c.Errs, appPkgs, allBinds)
	infraDesc := infra.ComputeDesc(p.c.Errs, appPkgs, allResources, allBinds, infraUsage)

	return Result{
		Packages: appPkgs,
		APIs:     apiResults,
		Infra:    infraDesc,
	}
}
