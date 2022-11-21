package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
)

const (
	encorePkgDir = "__encore"
	mainPkgName  = "main"
	etypePkgName = "etype"
)

func (b *builder) writeMainPkg() error {
	// Write the file to disk
	dir := filepath.Join(b.workdir, encorePkgDir, mainPkgName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	mainPath := filepath.Join(dir, "main.go")
	file, err := os.Create(mainPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(b.appRoot, encorePkgDir, mainPkgName, "main.go"), mainPath)

	f, err := b.codegen.Main(b.cfg.EncoreCompilerVersion, "", "")
	if err != nil {
		return err
	}
	return f.Render(file)
}

func (b *builder) writeEtypePkg() error {
	// Write the file to disk
	dir := filepath.Join(b.workdir, encorePkgDir, etypePkgName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, "etype.go")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(b.appRoot, encorePkgDir, etypePkgName, "etype.go"), filePath)

	f, err := b.codegen.Etype()
	if err != nil {
		return err
	}
	return f.Render(file)
}

func (b *builder) writeHandlers() error {
	for _, svc := range b.res.App.Services {
		if err := b.writeServiceHandlers(svc); err != nil {
			return fmt.Errorf("write handlers for svc %s: %v", svc.Name, err)
		}
	}

	for _, pkg := range b.res.App.Packages {
		if err := b.writePackageHandlers(pkg); err != nil {
			return fmt.Errorf("write handlers for pkg %s: %v", pkg.RelPath, err)
		}
	}

	return nil
}

func (b *builder) writeServiceHandlers(svc *est.Service) error {
	// Write the file to disk
	dir := filepath.Join(b.workdir, filepath.FromSlash(svc.Root.RelPath))
	name := "encore_internal__service.go"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, name)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(svc.Root.Dir, name), filePath)

	f, err := b.codegen.ServiceHandlers(svc)
	if err != nil {
		return err
	}
	return f.Render(file)
}

func (b *builder) writeConfigUnmarshallers() error {
	for _, svc := range b.res.App.Services {
		if len(svc.ConfigLoads) > 0 {
			if err := b.writeServiceConfigUnmarshalers(svc); err != nil {
				return eerror.Wrap(err, "compiler", "write config unmarshallers for svc", nil)
			}
		}
	}
	return nil
}

func (b *builder) writeServiceConfigUnmarshalers(svc *est.Service) error {
	// Write the file to disk
	dir := filepath.Join(b.workdir, filepath.FromSlash(svc.Root.RelPath))
	name := "encore_internal__config_unmarshalers.go"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, name)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(svc.Root.Dir, name), filePath)

	f, err := b.codegen.ConfigUnmarshalers(svc)
	if err != nil {
		return err
	}

	return f.Render(file)
}

func (b *builder) writePackageHandlers(pkg *est.Package) error {
	hasResources := false
	for _, file := range pkg.Files {
		if len(file.References) > 0 {
			hasResources = true
			break
		}
	}
	if !hasResources {
		return nil
	}

	// Write the file to disk
	dir := filepath.Join(b.workdir, filepath.FromSlash(pkg.RelPath))
	name := "encore_internal__package.go"

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, name)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(pkg.Dir, name), filePath)

	f, err := b.codegen.PackageResources(pkg)
	if err != nil {
		return err
	}
	return f.Render(file)
}

func (b *builder) generateTestMain(pkg *est.Package) (err error) {
	// Do nothing if the file contains no test files.
	isTestFile := func(f *est.File) bool { return strings.HasSuffix(f.Name, "_test.go") }
	if slices.IndexFunc(pkg.Files, isTestFile) == -1 {
		return nil
	}

	testMainPath := filepath.Join(b.workdir, filepath.FromSlash(pkg.RelPath), "encore_testmain_test.go")
	file, err := os.Create(testMainPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	f := b.codegen.TestMain(pkg, b.res.App.Services, b.EncoreEnvironmentalVariablesToEmbed())
	b.addOverlay(filepath.Join(pkg.Dir, "encore_testmain_test.go"), testMainPath)
	return f.Render(file)
}

func (b *builder) generateServiceSetup(svc *est.Service) (err error) {
	f := b.codegen.UserFacing(svc, true)
	if f == nil {
		return nil // nothing to do
	}

	encoreGenPath := filepath.Join(b.workdir, filepath.FromSlash(svc.Root.RelPath), "encore.gen.go")
	file, err := os.Create(encoreGenPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(svc.Root.Dir, "encore.gen.go"), encoreGenPath)
	return f.Render(file)
}

func (b *builder) generateCueFiles(svc *est.Service) (err error) {
	f, err := b.cuegen.UserFacing(svc)
	if f == nil || len(f) == 0 {
		return nil
	}

	dst := filepath.Join(b.workdir, filepath.FromSlash(svc.Root.RelPath), "encore.gen.cue")
	return os.WriteFile(dst, f, 0644)
}
