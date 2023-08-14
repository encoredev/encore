// Package build supports building and testing Encore applications
// with codegen and rewrite overlays.
package build

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"encore.dev/appruntime/exported/config"
	"encr.dev/internal/etrace"
	"encr.dev/pkg/errinsrc/srcerrors"
	"encr.dev/pkg/paths"
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

	// StaticConfig is the static config to embed into the binary.
	StaticConfig *config.Static
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

	workdir paths.FS
	// deleteWorkDir reports whether the workdir should be deleted.
	// It's true for temporarily generated workdirs.
	tempWorkDir bool
}

func (b *builder) Build() *Result {
	workdir, tempWorkDir := b.prepareWorkDir()
	b.workdir = workdir

	if tempWorkDir {
		defer func() {
			// If we have a bailout or any errors, delete the workdir.
			if _, ok := perr.CatchBailout(recover()); ok || b.errs.Len() > 0 {
				if !b.cfg.KeepOutput && workdir != "" {
					_ = os.RemoveAll(workdir.ToIO())
				}
			}
		}()
	}

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

		var ldflags strings.Builder
		b.writeStaticConfig(&ldflags)

		if b.cfg.Ctx.Build.StaticLink {
			// Enable external linking if we use cgo.
			if b.cfg.Ctx.Build.CgoEnabled {
				ldflags.WriteString(" -linkmode external")
			}

			ldflags.WriteString(` -extldflags "-static"`)
		}
		args = append(args, "-ldflags", ldflags.String())

		if b.cfg.Ctx.Build.Debug {
			// Disable inlining for better debugging.
			args = append(args, "-gcflags", "all=-N -l")
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
			"GOTOOLCHAIN=local",
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

func (b *builder) writeStaticConfig(ldflags *strings.Builder) {
	// Marshal the static config and add it as a linker flag.
	ldflags.WriteString("-X 'encore.dev/appruntime/shared/appconf.static=")
	data, err := json.Marshal(b.cfg.StaticConfig)
	if err != nil {
		b.errs.Fatalf(token.NoPos, "unable to marshal static config: %v", err)
	}
	ldflags.WriteString(base64.StdEncoding.EncodeToString(data))
	ldflags.WriteByte('\'')
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

func (b *builder) prepareWorkDir() (workdir paths.FS, temporary bool) {
	work, isTemp := (func() (string, bool) {
		// If we have an appID, use a persistent work dir.
		if appID, ok := b.cfg.Ctx.AppID.Get(); ok {
			baseDir, err := os.UserCacheDir()
			if err != nil {
				b.errs.Fatalf(token.NoPos, "unable to get user cache dir: %v", err)
			}
			workdir := filepath.Join(baseDir, "encore-build", appID)
			return workdir, false
		}

		tmp, err := os.MkdirTemp("", "encore-build")
		if err != nil {
			b.errs.Fatalf(token.NoPos, "unable to create workdir: %v", err)
		}
		return tmp, true
	})()

	// NOTE(andre): There appears to be a bug in go's handling of overlays
	// when the source or destination is a symlink.
	// I haven't dug into the root cause exactly, but it causes weird issues
	// with tests since macOS's /var/tmp is a symlink to /private/var/tmp.
	if d, err := filepath.EvalSymlinks(work); err == nil {
		work = d
	}

	if err := os.MkdirAll(work, 0o755); err != nil {
		b.errs.Fatalf(token.NoPos, "unable to create workdir: %v", err)
	}

	return paths.RootedFSPath(work, "."), isTemp
}
