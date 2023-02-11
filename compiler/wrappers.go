package compiler

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/compiler/internal/rewrite"
	"encr.dev/parser/est"
	"encr.dev/pkg/eerror"
)

const (
	encorePkgDir = "__encore"
	mainPkgName  = "main"
	etypePkgName = "etype"
)

func (b *builder) writeMainPkg() error {
	if b.disableAPI {
		return b.rewriteExistingMainPkg()
	}

	defer b.trace("write main package")()
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

	f, err := b.codegen.Main(b.cfg.EncoreCompilerVersion, "", "", true)
	if err != nil {
		return err
	}
	return f.Render(file)
}

func (b *builder) rewriteExistingMainPkg() error {
	defer b.trace("rewrite existing main package")()

	// TODO(andre) Need to make the actual main package path configurable.

	// Find the existing main package.
	var mainPkg *est.Package
	//mainPkgPath := b.cfg.ExecScript.ScriptMainPkg
	for _, pkg := range b.res.App.Packages {
		if pkg.Name == "main" {
			mainPkg = pkg
			break
		}
	}
	if mainPkg == nil {
		return fmt.Errorf("unable to find main package")
	}

	// Write the file to disk
	dir := filepath.Join(b.workdir, mainPkg.RelPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	mainPath := filepath.Join(dir, "encore_internal__main.go")
	file, err := os.Create(mainPath)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := file.Close(); err == nil {
			err = err2
		}
	}()

	b.addOverlay(filepath.Join(b.appRoot, mainPkg.RelPath, "exec_main.go"), mainPath)

	f, err := b.codegen.Main(b.cfg.EncoreCompilerVersion, mainPkg.ImportPath, "", false)
	if err != nil {
		return err
	}

	if err := f.Render(file); err != nil {
		return err
	}

	// Find the main function
	var (
		mainFunc *ast.FuncDecl
		mainFile *est.File
	)
FileLoop:
	for _, f := range mainPkg.Files {
		for _, d := range f.AST.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "main" {
				mainFile, mainFunc = f, fd
				break FileLoop
			}
		}
	}
	if mainFunc == nil {
		return fmt.Errorf("unable to find main function in %s", mainPkg.RelPath)
	}

	rw := rewrite.New(mainFile.Contents, mainFile.Token.Base())
	decl := mainFile.AST.Decls[0]
	ln := b.res.FileSet.Position(decl.Pos())
	rw.Insert(decl.Pos(), []byte(fmt.Sprintf("import __encore_app %s\n/*line :%d:%d*/", strconv.Quote("encore.dev/appruntime/app/appinit"), ln.Line, ln.Column)))
	rw.Insert(mainFunc.Body.Lbrace+1, []byte("__encore_app.AppStart();"))

	name := filepath.Base(mainFile.Path)
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, mainFile.Contents, 0644); err != nil {
		return err
	}
	b.addOverlay(mainFile.Path, dst)
	return nil
}

func (b *builder) writeEtypePkg() error {
	defer b.trace("write etype package")()
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

func (b *builder) serviceCodegen() error {
	if b.disableAPI {
		return nil
	}

	defer b.trace("write service codegen")()
	for _, svc := range b.res.App.Services {
		if err := b.writeServiceHandlers(svc); err != nil {
			return fmt.Errorf("write handlers for svc %s: %v", svc.Name, err)
		}
	}

	return nil
}

func (b *builder) infraCodegen() error {
	defer b.trace("write infra codegen")()

	for _, pkg := range b.res.App.Packages {
		if err := b.writeInfraCodegen(pkg); err != nil {
			return fmt.Errorf("write handlers for pkg %s: %v", pkg.RelPath, err)
		}
	}

	return nil
}

func (b *builder) writeInfraCodegen(pkg *est.Package) error {
	f, err := b.codegen.Infra(pkg)
	if err != nil {
		return err
	} else if f == nil {
		return nil
	}
	// Write the file to disk
	dir := filepath.Join(b.workdir, filepath.FromSlash(pkg.RelPath))
	name := "encore_internal__infra.go"

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
	return f.Render(file)
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
	defer b.trace("write config unmarshallers")()
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
