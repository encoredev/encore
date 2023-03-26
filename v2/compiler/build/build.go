// Package build supports building and testing Encore applications
// with codegen and rewrite overlays.
package build

import (
	"bytes"
	"context"
	"fmt"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"encr.dev/internal/etrace"
	"encr.dev/internal/paths"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/v2/internals/overlay"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
)

type Config struct {
	// Ctx controls the build.
	Ctx *parsectx.Context

	// Overlays describes the code generation overlays to apply,
	// in the form of rewritten files or generated files.
	Overlays []overlay.File

	// KeepOutput keeps the temporary build directory from being deleted in the case of failure.
	KeepOutput bool

	// Env are additional environment variables to set.
	Env []string

	// MainPkg is the main package to build.
	MainPkg paths.Pkg

	// NoBinary specifies that no binary should be built.
	// It's used if MainPkg specifies multiple packages,
	// which for example is the case when checking for compilation errors
	// without building a binary (such as during tests).
	NoBinary bool
}

type TestConfig struct {
	Config

	// Args are additional arguments to "go test".
	Args []string

	// Stdout specifies the stdout to use.
	Stdout io.Writer

	// Stderr specifies the stderr to use.
	Stderr io.Writer

	// WorkingDir is the working directory to invoke
	// the "go test" command from.
	WorkingDir paths.FS
}

type Result struct {
	Dir paths.FS
	Exe paths.FS
}

func Build(ctx context.Context, cfg *Config) *Result {
	b := &builder{
		ctx:  ctx,
		cfg:  cfg,
		errs: cfg.Ctx.Errs,
	}
	return b.Build()
}

func Test(ctx context.Context, cfg *TestConfig) {
	b := &builder{
		ctx:     ctx,
		cfg:     &cfg.Config,
		testCfg: cfg,
		errs:    cfg.Ctx.Errs,
	}
	b.Test()
}

type builder struct {
	// inputs
	ctx     context.Context
	cfg     *Config
	testCfg *TestConfig

	// internal state

	// errs is the error list to use.
	errs *perr.List

	// overlays are the additional overlay files to apply.
	overlays []overlay.File

	// workdir is the temporary workdir for the build.
	workdir paths.FS
}

func (b *builder) Build() *Result {
	workdir, err := os.MkdirTemp("", "encore-build")
	if err != nil {
		b.errs.AddStd(err)
	}

	// NOTE(andre): There appears to be a bug in go's handling of overlays
	// when the source or destination is a symlink.
	// I haven't dug into the root cause exactly, but it causes weird issues
	// with tests since macOS's /var/tmp is a symlink to /private/var/tmp.
	if d, err := filepath.EvalSymlinks(workdir); err == nil {
		workdir = d
	}

	b.workdir = paths.RootedFSPath(workdir, ".")

	defer func() {
		// If we have a bailout or any errors, delete the workdir.
		if _, ok := perr.CatchBailout(recover()); ok || b.errs.Len() > 0 {
			if !b.cfg.KeepOutput && workdir != "" {
				_ = os.RemoveAll(workdir)
			}
		}
	}()

	res := &Result{
		Dir: b.workdir,
		Exe: b.binaryPath(),
	}

	for _, fn := range []func(){
		b.writeModFile,
		b.writeSumFile,
		b.buildMain,
	} {
		fn()
		// Abort early if we encountered any errors.
		if b.errs.Len() > 0 {
			break
		}
	}
	return res
}

func (b *builder) Test() {
	workdir, err := os.MkdirTemp("", "encore-test")
	if err != nil {
		b.errs.AddStd(err)
		return
	}
	// NOTE(andre): There appears to be a bug in go's handling of overlays
	// when the source or destination is a symlink.
	// I haven't dug into the root cause exactly, but it causes weird issues
	// with tests since macOS's /var/tmp is a symlink to /private/var/tmp.
	if d, err := filepath.EvalSymlinks(workdir); err == nil {
		workdir = d
	}
	b.workdir = paths.RootedFSPath(workdir, ".")

	defer func() {
		// If we have a bailout or any errors, delete the workdir.
		if _, ok := perr.CatchBailout(recover()); ok || b.errs.Len() > 0 {
			if !b.cfg.KeepOutput && workdir != "" {
				_ = os.RemoveAll(workdir)
			}
		}
	}()

	for _, fn := range []func(){
		b.writeModFile,
		b.writeSumFile,
		b.runTests,
	} {
		fn()
		// Abort early if we encountered any errors.
		if b.errs.Len() > 0 {
			break
		}
	}
}

func (b *builder) writeModFile() {
	etrace.Sync0(b.ctx, "", "writeModFile", func(ctx context.Context) {
		newPath := b.cfg.Ctx.Build.EncoreRuntime.ToIO()
		oldPath := "encore.dev"

		modData, err := os.ReadFile(b.cfg.Ctx.MainModuleDir.Join("go.mod").ToIO())
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to read go.mod: %v", err)
			return
		}
		mainMod, err := modfile.Parse("go.mod", modData, nil)
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to parse go.mod: %v", err)
			return
		}

		// Make sure there's a dependency on encore.dev so it can be replaced.
		if err := mainMod.AddRequire("encore.dev", "v0.0.0"); err != nil {
			b.errs.Addf(token.NoPos, "unable to add 'require encore.dev' directive to go.mod: %v", err)
			return
		}
		if err := mainMod.AddReplace(oldPath, "", newPath, ""); err != nil {
			b.errs.Addf(token.NoPos, "unable to add 'replace encore.dev' directive to go.mod: %v", err)
			return
		}

		// We require Go 1.18+ now that we use generics in code gen.
		if !isGo118Plus(mainMod) {
			_ = mainMod.AddGoStmt("1.18")
		}
		mainMod.Cleanup()

		runtimeModData, err := os.ReadFile(filepath.Join(newPath, "go.mod"))
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to read encore runtime's go.mod: %v", err)
			return
		}
		runtimeModfile, err := modfile.Parse("encore-runtime/go.mod", runtimeModData, nil)
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to parse encore runtime's go.mod: %v", err)
			return
		}
		mergeModfiles(mainMod, runtimeModfile)

		data := modfile.Format(mainMod.Syntax)
		if err := os.WriteFile(b.gomodPath().ToIO(), data, 0o644); err != nil {
			b.errs.Addf(token.NoPos, "unable to write go.mod: %v", err)
			return
		}
	})
}

func (b *builder) writeSumFile() {
	etrace.Sync0(b.ctx, "", "writeSumFile", func(ctx context.Context) {
		appSum, err := os.ReadFile(b.cfg.Ctx.MainModuleDir.Join("go.sum").ToIO())
		if err != nil && !os.IsNotExist(err) {
			b.errs.Addf(token.NoPos, "unable to parse go.sum: %v", err)
			return
		}
		runtimeSum, err := os.ReadFile(b.cfg.Ctx.Build.EncoreRuntime.Join("go.sum").ToIO())
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to parse encore runtime's go.sum: %v", err)
			return
		}
		if !bytes.HasSuffix(appSum, []byte{'\n'}) {
			appSum = append(appSum, '\n')
		}
		data := append(appSum, runtimeSum...)

		if err := os.WriteFile(b.gosumPath().ToIO(), data, 0o644); err != nil {
			b.errs.Addf(token.NoPos, "unable to write go.sum: %v", err)
			return
		}
	})
}

func (b *builder) gomodPath() paths.FS { return b.workdir.Join("go.mod") }
func (b *builder) gosumPath() paths.FS { return b.workdir.Join("go.sum") }

func (b *builder) buildMain() {
	etrace.Sync0(b.ctx, "", "buildMain", func(ctx context.Context) {
		overlayFiles := append(b.overlays, b.cfg.Overlays...)
		overlayPath, err := overlay.Write(b.workdir, overlayFiles)
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to write overlay file: %v", err)
			return
		}

		build := b.cfg.Ctx.Build
		tags := append([]string{"encore", "encore_internal", "encore_app"}, build.BuildTags...)
		args := []string{
			"build",
			"-tags=" + strings.Join(tags, ","),
			"-overlay=" + overlayPath.ToIO(),
			"-modfile=" + b.gomodPath().ToIO(),
			"-mod=mod",
		}

		if !b.cfg.NoBinary {
			args = append(args, "-o="+b.binaryPath().ToIO())
		}

		if b.cfg.Ctx.Build.StaticLink {
			var ldflags string

			// Enable external linking if we use cgo.
			if b.cfg.Ctx.Build.CgoEnabled {
				ldflags = "-linkmode external "
			}

			ldflags += `-extldflags "-static"`
			args = append(args, "-ldflags", ldflags)
		}

		if b.cfg.Ctx.Build.Debug {
			// Disable inlining for better debugging.
			args = append(args, `-gcflags "all=-N -l"`)
		}

		args = append(args, b.cfg.MainPkg.String())

		goroot := build.GOROOT
		cmd := exec.Command(goroot.Join("bin", "go"+b.exe()).ToIO(), args...)

		// Copy the env before we add additional env vars
		// to avoid accidentally sharing the same backing array.
		env := make([]string, len(b.cfg.Env))
		copy(env, b.cfg.Env)

		env = append(env,
			"GO111MODULE=on",
			"GOROOT="+goroot.ToIO(),
		)
		if goos := build.GOOS; goos != "" {
			env = append(env, "GOOS="+goos)
		}
		if goarch := build.GOARCH; goarch != "" {
			env = append(env, "GOARCH="+goarch)
		}
		if !build.CgoEnabled {
			env = append(env, "CGO_ENABLED=0")
		}
		cmd.Env = append(os.Environ(), env...)
		cmd.Dir = b.cfg.Ctx.MainModuleDir.ToIO()
		out, err := cmd.CombinedOutput()
		if err != nil {
			if len(out) == 0 {
				out = []byte(err.Error())
			}
			out = convertCompileErrors(b.errs, out, b.workdir.ToIO(), b.cfg.Ctx.MainModuleDir.ToIO(), b.cfg.Ctx.MainModuleDir.ToIO())
			if len(out) > 0 {
				// HACK(andre): Make this nicer
				b.errs.AddStd(fmt.Errorf("compilation failure: %s", out))
			}
		}
	})
}

func (b *builder) runTests() {
	etrace.Sync0(b.ctx, "", "runTests", func(ctx context.Context) {
		overlayFiles := append(b.overlays, b.cfg.Overlays...)
		overlayPath, err := overlay.Write(b.workdir, overlayFiles)
		if err != nil {
			b.errs.Addf(token.NoPos, "unable to write overlay file: %v", err)
			return
		}

		build := b.cfg.Ctx.Build
		tags := append([]string{"encore", "encore_internal", "encore_app"}, build.BuildTags...)
		args := []string{
			"test",
			"-tags=" + strings.Join(tags, ","),
			"-overlay=" + overlayPath.ToIO(),
			"-modfile=" + b.gomodPath().ToIO(),
			"-mod=mod",
			"-vet=off",
		}

		if b.cfg.Ctx.Build.StaticLink {
			var ldflags string

			// Enable external linking if we use cgo.
			if b.cfg.Ctx.Build.CgoEnabled {
				ldflags = "-linkmode external "
			}

			ldflags += `-extldflags "-static"`
			args = append(args, "-ldflags", ldflags)
		}

		if b.cfg.Ctx.Build.Debug {
			// Disable inlining for better debugging.
			args = append(args, `-gcflags "all=-N -l"`)
		}

		args = append(args, b.testCfg.Args...)

		goroot := build.GOROOT
		cmd := exec.CommandContext(b.cfg.Ctx.Ctx, goroot.Join("bin", "go"+b.exe()).ToIO(), args...)

		// Copy the env before we add additional env vars
		// to avoid accidentally sharing the same backing array.
		env := make([]string, len(b.cfg.Env))
		copy(env, b.cfg.Env)

		env = append(env,
			"GO111MODULE=on",
			"GOROOT="+goroot.ToIO(),
		)
		if goos := build.GOOS; goos != "" {
			env = append(env, "GOOS="+goos)
		}
		if goarch := build.GOARCH; goarch != "" {
			env = append(env, "GOARCH="+goarch)
		}
		if !build.CgoEnabled {
			env = append(env, "CGO_ENABLED=0")
		}
		cmd.Env = append(os.Environ(), env...)
		cmd.Dir = b.testCfg.WorkingDir.ToIO()
		cmd.Stdout = b.testCfg.Stdout
		cmd.Stderr = b.testCfg.Stderr

		err = cmd.Run()
		if err != nil {
			// HACK(andre): Make this nicer
			b.errs.AddStd(fmt.Errorf("test failure: %v", err))
		}
	})
}

// mergeModFiles merges two modfiles, adding the require statements from the latter to the former.
// If both files have the same module requirement, it keeps the one with the greater semver version.
func mergeModfiles(src, add *modfile.File) {
	reqs := src.Require
	for _, a := range add.Require {
		found := false
		for _, r := range src.Require {
			if r.Mod.Path == a.Mod.Path {
				found = true
				// Update the version if the one to add is greater.
				if semver.Compare(a.Mod.Version, r.Mod.Version) > 0 {
					r.Mod.Version = a.Mod.Version
				}
			}
		}
		if !found {
			reqs = append(reqs, a)
		}
	}
	src.SetRequire(reqs)
	src.Cleanup()
}

const binaryName = "encore_app_out"

func (b *builder) exe() string {
	goos := b.cfg.Ctx.Build.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

func (b *builder) binaryPath() paths.FS {
	return b.workdir.Join(binaryName + b.exe())
}

// convertCompileErrors goes through the errors and converts basic compiler errors into
// ErrInSrc errors, which are more useful for the user.
func convertCompileErrors(errList *perr.List, out []byte, workdir, appRoot, relwd string) []byte {
	wdroot := filepath.Join(appRoot, relwd)
	lines := bytes.Split(out, []byte{'\n'})
	prefix := append([]byte(workdir), '/')
	modified := false

	output := make([][]byte, 0)

	for _, line := range lines {
		if !bytes.HasPrefix(line, prefix) {
			output = append(output, line)
			continue
		}
		idx := bytes.IndexByte(line, ':')
		if idx == -1 || idx < len(prefix) {
			output = append(output, line)
			continue
		}

		filename := line[:idx]
		appPath := filepath.Join(appRoot, string(filename[len(prefix):]))
		if _, err := filepath.Rel(wdroot, appPath); err == nil {
			parts := strings.SplitN(string(line), ":", 4)
			if len(parts) != 4 {
				output = append(output, line)
				continue
			}

			lineNumber, err := strconv.Atoi(parts[1])
			if err != nil {
				output = append(output, line)
				continue
			}

			colNumber, err := strconv.Atoi(parts[2])
			if err != nil {
				output = append(output, line)
				continue
			}

			modified = true
			errList.AddStd(srcerrors.GenericGoCompilerError(changeToAppRootFile(parts[0], workdir, appRoot), lineNumber, colNumber, parts[3]))
		} else {
			output = append(output, line)
		}
	}

	if !modified {
		return out
	}

	// Append the err list for both the workDir and the appRoot
	// as files might be coming from either of them
	errList.MakeRelative(workdir, relwd)
	errList.MakeRelative(appRoot, relwd)
	output = append(output)

	return bytes.Join(output, []byte{'\n'})
}

// changeToAppRootFile will return the compiledFile path inside the appRoot directory
// if that file exists within the app root. Otherwise it will return the original
// compiledFile path.
//
// This means when we display compiler errors to the user, we will show them their
// original rewritten code, where line numbers and column numbers will align with
// their own code.
//
// However for generated files which don't exist in their own folders, we will
// still be able to render the source causing the issue
func changeToAppRootFile(compiledFile string, workDirectory, appRoot string) string {
	if strings.HasPrefix(compiledFile, workDirectory) {
		fileInOriginalSrc := strings.TrimPrefix(compiledFile, workDirectory)
		fileInOriginalSrc = path.Join(appRoot, fileInOriginalSrc)

		if _, err := os.Stat(fileInOriginalSrc); err == nil {
			return fileInOriginalSrc
		}
	}

	return compiledFile
}

func isGo118Plus(f *modfile.File) bool {
	if f.Go == nil {
		return false
	}
	m := modfile.GoVersionRE.FindStringSubmatch(f.Go.Version)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 1 || (major == 1 && minor >= 18)
}
