package compiler

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"encr.dev/internal/optracker"
	"encr.dev/parser/est"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/errlist"
)

type ExecScriptConfig struct {
	// Env sets environment variables for "go test".
	Env []string

	// Args sets extra arguments for "go test".
	Args []string

	// Stdout and Stderr are where to redirect "go test" output.
	Stdout, Stderr io.Writer
}

// ExecScript executes a one-off script.
//
// On success, it is the caller's responsibility to delete the temp dir
// returned in Result.Dir.
func ExecScript(appRoot string, cfg *Config) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	} else if appRoot, err = filepath.Abs(appRoot); err != nil {
		return nil, err
	}

	b := &builder{
		cfg:      cfg,
		appRoot:  appRoot,
		configs:  make(map[string]string),
		lastOpID: optracker.NoOperationID,
	}
	return b.ExecScript()
}

func (b *builder) ExecScript() (res *Result, err error) {
	defer func() {
		if e := recover(); e != nil {
			if b, ok := e.(bailout); ok {
				err = b.err
			} else {
				err = srcerrors.UnhandledPanic(e)
			}

			if list := errlist.Convert(err); list != nil {
				list.MakeRelative(b.appRoot, "")
				err = list
			}

			b.cfg.OpTracker.Fail(b.lastOpID, err)
		}
	}()

	b.workdir, err = ioutil.TempDir("", "encore-exec")
	if err != nil {
		return nil, err
	}

	res = &Result{
		Dir: b.workdir,
		Exe: filepath.Join(b.workdir, binaryName+b.exe()),
	}
	defer func() {
		if err != nil && !b.cfg.KeepOutput {
			os.RemoveAll(b.workdir)
		}
	}()

	for _, fn := range []func() error{
		b.parseApp,
		b.pickupConfigFiles,
		b.checkApp, // we need to validate & compute the config
		b.writeModFile,
		b.writeSumFile,
		b.writePackages,
		b.writeHandlers,
		b.writeConfigUnmarshallers,
		b.writeEtypePkg,
		b.writeExecMain,
		b.buildExecScript,
	} {
		if err := fn(); err != nil {
			b.cfg.OpTracker.Fail(b.lastOpID, err)
			return res, err
		}
	}

	res.Parse = b.res
	res.ConfigFiles = b.configFiles
	res.Configs = b.configs
	return res, nil
}

func (b *builder) writeExecMain() error {
	// Find the main package in question
	mainPkgPath := filepath.Clean(b.cfg.WorkingDir)
	var mainPkg *est.Package
	for _, pkg := range b.res.App.Packages {
		if pkg.RelPath == mainPkgPath {
			mainPkg = pkg
			break
		}
	}
	if mainPkg == nil {
		return fmt.Errorf("unable to find main package: %s", mainPkgPath)
	}

	// Write the file to disk
	dir := filepath.Join(b.workdir, mainPkg.RelPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	mainPath := filepath.Join(dir, "encore_internal__exec_main.go")
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

	f, err := b.codegen.Main(b.cfg.EncoreCompilerVersion, mainPkg.ImportPath, "encoreInternal_ExecMain")
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
		return fmt.Errorf("unable to find main function in %s", mainPkgPath)
	}

	//rw := rewrite.New(mainFile.Contents, mainFile.Token.Base())
	//rw.Insert(mainFunc.Body.Lbrace+1, []byte("encoreInternal_ExecMain();"))

	name := filepath.Base(mainFile.Path)
	dst := filepath.Join(dir, name)
	if err := ioutil.WriteFile(dst, mainFile.Contents, 0644); err != nil {
		return err
	}
	b.addOverlay(mainFile.Path, dst)
	return nil
}

// buildExecScript builds the exec script.
func (b *builder) buildExecScript() error {
	compileAppOpId := b.cfg.OpTracker.Add("Compiling application source code", time.Now())
	b.lastOpID = compileAppOpId

	overlayData, _ := json.Marshal(map[string]interface{}{"Replace": b.overlay})
	overlayPath := filepath.Join(b.workdir, "overlay.json")
	if err := ioutil.WriteFile(overlayPath, overlayData, 0644); err != nil {
		return err
	}

	binName := filepath.Join(b.workdir, binaryName+b.exe())
	tags := append([]string{"encore", "encore_internal", "encore_app"}, b.cfg.BuildTags...)
	args := []string{
		"build",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + overlayPath,
		"-modfile=" + filepath.Join(b.workdir, "go.mod"),
		"-mod=mod",
		"-o=" + binName,
	}

	if b.cfg.StaticLink {
		var ldflags string

		// Enable external linking if we use cgo.
		if b.cfg.CgoEnabled {
			ldflags = "-linkmode external "
		}

		ldflags += `-extldflags "-static"`
		args = append(args, "-ldflags", ldflags)
	}

	cmd := exec.Command(filepath.Join(b.cfg.EncoreGoRoot, "bin", "go"+b.exe()), args...)

	env := []string{
		"GO111MODULE=on",
		"GOROOT=" + b.cfg.EncoreGoRoot,
	}
	if !b.cfg.CgoEnabled {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = filepath.Join(b.appRoot, b.cfg.WorkingDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) == 0 {
			out = []byte(err.Error())
		}
		out = convertCompileErrors(out, b.workdir, b.appRoot, b.cfg.WorkingDir)
		return &Error{Output: out}
	}

	b.cfg.OpTracker.Done(compileAppOpId, 300*time.Millisecond)
	return nil
}
