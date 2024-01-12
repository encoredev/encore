// Package build supports building and testing Encore applications
// with codegen and rewrite overlays.
package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"encr.dev/internal/etrace"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/perr"
)

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

func Test(ctx context.Context, cfg *TestConfig) {
	b := &builder{
		ctx:     ctx,
		cfg:     &cfg.Config,
		testCfg: cfg,
		mode:    testMode,
		errs:    cfg.Ctx.Errs,
	}
	b.Test()
}

func (b *builder) Test() {
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

	if b.cfg.KeepOutput && b.workdir != "" {
		_, _ = fmt.Fprintf(b.testCfg.Stdout, "wrote generated code to: %s\n", b.workdir.ToIO())
	}

	for _, fn := range []func(){
		b.writeModFile,
		b.runTests,
	} {
		fn()
		// Abort early if we encountered any errors.
		if b.errs.Len() > 0 {
			break
		}
	}
}

func (b *builder) runTests() {
	etrace.Sync0(b.ctx, "", "runTests", func(ctx context.Context) {
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
		cmd.Dir = b.testCfg.WorkingDir.ToIO()
		cmd.Stdout = b.testCfg.Stdout
		cmd.Stderr = b.testCfg.Stderr

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
