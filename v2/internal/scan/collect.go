package scan

import (
	"fmt"
	"os"

	"golang.org/x/mod/modfile"

	"encr.dev/internal/paths"
	"encr.dev/v2/internal/perr"
	"encr.dev/v2/internal/pkginfo"
)

// ProcessModule parses all the packages in the module located at modRoot.
// It calls process for each package. Multiple goroutines may call process
// concurrently.
func ProcessModule(errs *perr.List, loader *pkginfo.Loader, moduleRoot paths.FS, process func(pkg *pkginfo.Package)) {
	// Resolve the module path for the main module.
	modFilePath := moduleRoot.Join("go.mod")
	modPath, err := resolveModulePath(modFilePath)
	if err != nil {
		errs.Add(errResolvingModulePath.InFile(modFilePath.ToIO()).Wrapping(err))
		return
	}

	quit := make(chan struct{})
	defer close(quit)
	pkgCh := Packages(quit, errs, loader, moduleRoot, paths.Pkg(modPath))

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
