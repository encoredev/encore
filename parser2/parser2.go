package parser2

import (
	"context"
	"fmt"
	"go/token"
	"os"

	"github.com/rs/zerolog"
	"golang.org/x/mod/modfile"

	"encr.dev/parser2/apis"
	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/paths"
	"encr.dev/parser2/internal/perr"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/pkg/experiments"
)

// Config represents the configuration options for parsing.
type Config struct {
	// Ctx provides cancellation.
	Ctx context.Context

	// Log is the configured logger.
	Log zerolog.Logger

	// Build controls what files to build.
	Build parsectx.BuildInfo

	// Errs is the error list to use.
	Errs *perr.List

	// Experiments are the experiments to enable.
	Experiments *experiments.Set

	// RootDir is the root directory to parse.
	RootDir string

	// ParseTests controls whether to parse test files.
	ParseTests bool
}

func NewParser(cfg *Config) *Parser {
	// Currently we assume the go.mod file is in the root directory.
	mainModuleDir := paths.RootedFSPath(cfg.RootDir, ".")

	c := &parsectx.Context{
		Ctx:           cfg.Ctx,
		Log:           cfg.Log,
		Build:         cfg.Build,
		MainModuleDir: mainModuleDir,
		FS:            token.NewFileSet(),
		ParseTests:    cfg.ParseTests,
		Errs:          cfg.Errs,
	}
	return NewParserFromCtx(c)
}

func NewParserFromCtx(c *parsectx.Context) *Parser {
	loader := pkginfo.New(c)
	schemaParser := schema.NewParser(c, loader)
	apiParser := apis.NewParser(c, schemaParser)
	return &Parser{
		c:         c,
		loader:    loader,
		apiParser: apiParser,
	}
}

type Parser struct {
	c         *parsectx.Context
	loader    *pkginfo.Loader
	apiParser *apis.Parser
}

func (p *Parser) Parse() {
	p.collectPackages(p.processPkg)
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

// processPkg processes a single package.
func (p *Parser) processPkg(pkg *pkginfo.Package) {
	res := p.apiParser.Parse(pkg)
	p.c.Log.Info().Msgf("package %s: -> %+v", pkg.ImportPath, res)
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
