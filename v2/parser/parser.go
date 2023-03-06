package parser

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/mod/modfile"

	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/paths"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/parser/apis"
	"encr.dev/v2/parser/infra"
	"encr.dev/v2/parser/infra/resource"
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

type Result struct {
	APIs           []*apis.ParseResult
	InfraResources []resource.Resource
}

func (p *Parser) Parse() Result {
	var (
		mu           sync.Mutex
		allResources []resource.Resource
		apiResults   []*apis.ParseResult
	)

	p.collectPackages(func(pkg *pkginfo.Package) {
		apiRes := p.apiParser.Parse(pkg)
		resources := p.infraParser.Parse(pkg)

		mu.Lock()
		apiResults = append(apiResults, apiRes)
		allResources = append(allResources, resources...)
		mu.Unlock()
	})

	return Result{
		InfraResources: allResources,
		APIs:           apiResults,
	}
}

// collectPackages parses all the packages in subdirectories of the root directory.
// It calls process for each package. Multiple goroutines may call process
// concurrently.
func (p *Parser) collectPackages(process func(pkg *pkginfo.Package)) {
	// Resolve the module path for the main module.
	modFilePath := p.c.MainModuleDir.Join("go.mod")
	modPath, err := resolveModulePath(modFilePath)
	if err != nil {
		p.c.Errs.AddForFile(err, modFilePath.ToIO())
		return
	}

	quit := make(chan struct{})
	defer close(quit)
	pkgCh := scanPackages(quit, p.c.Errs, p.loader, p.c.MainModuleDir, paths.Pkg(modPath))

	for pkg := range pkgCh {
		process(pkg)
	}
}

// resolveModulePath resolves the main module's module path
// by reading the go.mod file at goModPath.
func resolveModulePath(goModPath paths.FS) (paths.Mod, error) {
	data, err := os.ReadFile(goModPath.ToIO())
	if err != nil {
		return "", err
	}
	modFile, err := modfile.Parse(goModPath.ToDisplay(), data, nil)
	if err != nil {
		return "", err
	}
	if !paths.ValidModPath(modFile.Module.Mod.Path) {
		return "", fmt.Errorf("invalid module path %q", modFile.Module.Mod.Path)
	}
	return paths.MustModPath(modFile.Module.Mod.Path), nil
}
