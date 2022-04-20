package compiler

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TestConfig struct {
	// Env sets environment variables for "go test".
	Env []string

	// Args sets extra arguments for "go test".
	Args []string

	// Stdout and Stderr are where to redirect "go test" output.
	Stdout, Stderr io.Writer
}

// Test tests the application.
//
// On success, it is the caller's responsibility to delete the temp dir
// returned in Result.Dir.
func Test(ctx context.Context, appRoot string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	} else if appRoot, err = filepath.Abs(appRoot); err != nil {
		return err
	}

	b := &builder{
		cfg:        cfg,
		appRoot:    appRoot,
		parseTests: true,
	}
	return b.Test(ctx)
}

func (b *builder) Test(ctx context.Context) (err error) {
	defer func() {
		if e := recover(); e != nil {
			if b, ok := e.(bailout); ok {
				err = b.err
			} else {
				panic(e)
			}
		}
	}()

	b.workdir, err = ioutil.TempDir("", "encore-test")
	if err != nil {
		return err
	}
	defer os.RemoveAll(b.workdir)

	for _, fn := range []func() error{
		b.parseApp,
		b.writeModFile,
		b.writeSumFile,
		b.writePackages,
		b.writeTestMains,
	} {
		if err := fn(); err != nil {
			return err
		}
	}
	return b.runTests(ctx)
}

func (b *builder) writeTestMains() error {
	for _, pkg := range b.res.App.Packages {
		if err := b.generateTestMain(pkg); err != nil {
			return err
		}
	}
	return nil
}

// runTests runs "go test".
func (b *builder) runTests(ctx context.Context) error {
	overlayData, _ := json.Marshal(map[string]interface{}{"Replace": b.overlay})
	overlayPath := filepath.Join(b.workdir, "overlay.json")
	if err := ioutil.WriteFile(overlayPath, overlayData, 0644); err != nil {
		return err
	}

	tags := append([]string{"encore"}, b.cfg.BuildTags...)
	args := []string{
		"test",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + overlayPath,
		"-modfile=" + filepath.Join(b.workdir, "go.mod"),
		"-mod=mod",
		"-vet=off",
	}
	if b.cfg.StaticLink {
		args = append(args, "-ldflags", `-extldflags "-static"`)
	}
	args = append(args, b.cfg.Test.Args...)
	cmd := exec.CommandContext(ctx, filepath.Join(b.cfg.EncoreGoRoot, "bin", "go"+exe), args...)
	env := append(b.cfg.Test.Env,
		"GO111MODULE=on",
		"GOROOT="+b.cfg.EncoreGoRoot,
	)
	if !b.cfg.CgoEnabled {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = filepath.Join(b.appRoot, b.cfg.WorkingDir)
	cmd.Stdout = b.cfg.Test.Stdout
	cmd.Stderr = b.cfg.Test.Stderr
	return cmd.Run()
}
