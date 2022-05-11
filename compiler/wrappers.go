package compiler

import (
	"os"
	"path/filepath"

	"encr.dev/compiler/internal/codegen"
	"encr.dev/parser/est"
)

const mainPkgName = "__encore_main"

func (b *builder) writeMainPkg() error {
	// Write the file to disk
	dir := filepath.Join(b.workdir, mainPkgName)
	if err := os.Mkdir(dir, 0755); err != nil {
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

	b.addOverlay(filepath.Join(b.appRoot, mainPkgName, "main.go"), mainPath)

	mb := codegen.NewBuilder(b.res, b.cfg.EncoreCompilerVersion)
	f, err := mb.Main()
	if err != nil {
		return err
	}
	return f.Render(file)
}

func (b *builder) generateWrappers(pkg *est.Package, rpcs []*est.RPC, wrapperPath string) (err error) {
	file, err := os.Create(wrapperPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	mb := codegen.NewBuilder(b.res, b.cfg.EncoreCompilerVersion)
	f := mb.Wrappers(pkg, rpcs)
	return f.Render(file)
}

func (b *builder) generateTestMain(pkg *est.Package) (err error) {
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

	mb := codegen.NewBuilder(b.res, b.cfg.EncoreCompilerVersion)
	f := mb.TestMain(pkg, b.res.App.Services)
	b.addOverlay(filepath.Join(pkg.Dir, "encore_testmain_test.go"), testMainPath)
	return f.Render(file)
}
