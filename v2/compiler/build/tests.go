// Package build supports building and testing Encore applications
// with codegen and rewrite overlays.
package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"encr.dev/internal/etrace"
	builderpkg "encr.dev/pkg/builder"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/perr"
)

type TestConfig struct {
	GenerateTestSpecConfig
	RunTestsConfig
}

func Test(ctx context.Context, cfg *TestConfig) {
	b := &builder{
		ctx:  ctx,
		cfg:  &cfg.Config,
		mode: testMode,
		errs: cfg.Ctx.Errs,
	}
	spec := b.GenerateTestSpec(&cfg.GenerateTestSpecConfig)
	b.RunTests(spec, &cfg.RunTestsConfig)
}

type TestSpec struct {
	Command string
	Args    []string
	Environ []string

	cfg *Config
}

type GenerateTestSpecConfig struct {
	Config

	// Args are additional arguments to "go test".
	Args []string
}

func GenerateTestSpec(ctx context.Context, cfg *GenerateTestSpecConfig) *TestSpec {
	b := &builder{
		ctx:  ctx,
		cfg:  &cfg.Config,
		mode: testMode,
		errs: cfg.Ctx.Errs,
	}
	return b.GenerateTestSpec(cfg)
}

func RunTests(ctx context.Context, spec *TestSpec, cfg *RunTestsConfig) {
	b := &builder{
		ctx:  ctx,
		cfg:  spec.cfg,
		mode: testMode,
		errs: spec.cfg.Ctx.Errs,
	}
	b.RunTests(spec, cfg)
}

type RunTestsConfig struct {
	// Stdout specifies the stdout to use.
	Stdout io.Writer

	// Stderr specifies the stderr to use.
	Stderr io.Writer

	// WorkingDir is the working directory to invoke
	// the "go test" command from.
	WorkingDir paths.FS
}

func (b *builder) RunTests(spec *TestSpec, testCfg *RunTestsConfig) {
	if b.cfg.KeepOutput && b.workdir != "" {
		_, _ = fmt.Fprintf(testCfg.Stdout, "wrote generated code to: %s\n", b.workdir.ToIO())
	}
	b.runTests(spec, testCfg)
}

func (b *builder) GenerateTestSpec(cfg *GenerateTestSpecConfig) *TestSpec {
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

	for _, fn := range []func(){
		b.writeModFile,
	} {
		fn()
		// Abort early if we encountered any errors.
		if b.errs.Len() > 0 {
			break
		}
	}

	return b.generateTestSpec(cfg)
}

func (b *builder) runTests(spec *TestSpec, testCfg *RunTestsConfig) {
	etrace.Sync0(b.ctx, "", "runTests", func(ctx context.Context) {
		cmd := exec.CommandContext(b.cfg.Ctx.Ctx, spec.Command, spec.Args...)
		cmd.Env = spec.Environ
		cmd.Dir = testCfg.WorkingDir.ToIO()
		cmd.Stdout = testCfg.Stdout
		cmd.Stderr = testCfg.Stderr

		err := cmd.Run()
		if err != nil {
			if err.Error() == "exit status 1" {
				// This is a standard error code for failed tests.
				// so we don't need to wrap it.
				b.errs.Add(ErrTestFailed)
			} else {
				// Otherwise we wrap the error.
				b.errs.Add(ErrTestFailed.Wrapping(err))
			}
		}
	})
}

func (b *builder) generateTestSpec(testCfg *GenerateTestSpecConfig) *TestSpec {
	build := b.cfg.Ctx.Build
	tags := append([]string{"encore", "encore_internal", "encore_app"}, build.BuildTags...)
	args := []string{
		"test",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + b.overlayPath.ToIO(),
		"-vet=off",
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

	if b.cfg.Ctx.Build.Debug != builderpkg.DebugModeDisabled {
		// Disable inlining for better debugging.
		args = append(args, `-gcflags "all=-N -l"`)
	}

	args = append(args, testCfg.Args...)

	goroot := build.GOROOT

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

	environ := append(os.Environ(), env...)

	// Get the last PATH= item
	var originalPath string
	for _, e := range environ {
		if path, ok := strings.CutPrefix(e, "PATH="); ok {
			originalPath = path
		}
	}

	// prefix PATH with encore-go, so it doesnt conflict with other installed go versions
	// if not set causes problems when running cover tests in go
	path := goroot.Join("bin").ToIO() + string(filepath.ListSeparator) + originalPath
	environ = append(environ, "PATH="+path)

	return &TestSpec{
		Command: goroot.Join("bin", "go"+b.exe()).ToIO(),
		Args:    args,
		Environ: environ,
		cfg:     b.cfg,
	}
}
