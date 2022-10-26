package compiler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"

	"encr.dev/pkg/errinsrc/srcerrors"
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
		forTesting: true,
		configs:    make(map[string]string),
	}
	return b.Test(ctx)
}

func (b *builder) Test(ctx context.Context) (err error) {
	defer func() {
		if e := recover(); e != nil {
			if b, ok := e.(bailout); ok {
				err = b.err
			} else {
				err = srcerrors.UnhandledPanic(e)
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
		b.pickupConfigFiles,
		b.checkApp, // we need to validate & compute the config
		b.writeModFile,
		b.writeSumFile,
		b.writePackages,
		b.writeHandlers,
		b.writeTestMains,
		b.writeConfigUnmarshallers,
		b.writeEtypePkg,
	} {
		if err := fn(); err != nil {
			return err
		}
	}
	return b.runTests(ctx)
}

// EncoreEnvironmentalVariablesToEmbed tells us if we need to embed the environmental variables into the built
// binary for testing.
//
// This is needed because GoLand first builds the test binary as one phase, and then secondary executes that built binary
func (b *builder) EncoreEnvironmentalVariablesToEmbed() []string {
	if b.forTesting == false || b.cfg.Test == nil {
		return nil
	}

	// If -c is passed to the go test, it means compile the test binary to pkg.test but do not run it
	if !slices.Contains(b.cfg.Test.Args, "-c") {
		return nil
	}

	rtn := make([]string, 0)
	for _, env := range b.cfg.Test.Env {
		if strings.HasPrefix(env, "ENCORE_") {
			rtn = append(rtn, env)
		}
	}

	// Embed any computed configs
	for serviceName, cfgString := range b.configs {
		rtn = append(rtn, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
	}
	return rtn
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

	tags := append([]string{"encore", "encore_internal", "encore_app"}, b.cfg.BuildTags...)
	args := []string{
		"test",
		"-tags=" + strings.Join(tags, ","),
		"-overlay=" + overlayPath,
		"-modfile=" + filepath.Join(b.workdir, "go.mod"),
		"-mod=mod",
		"-vet=off",
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

	args = append(args, b.cfg.Test.Args...)
	cmd := exec.CommandContext(ctx, filepath.Join(b.cfg.EncoreGoRoot, "bin", "go"+b.exe()), args...)

	// Copy the env before we add additional env vars
	// to avoid accidentally sharing the same backing array.
	env := make([]string, len(b.cfg.Test.Env))
	copy(env, b.cfg.Test.Env)
	env = append(env,
		"GO111MODULE=on",
		"GOROOT="+b.cfg.EncoreGoRoot,
	)
	if !b.cfg.CgoEnabled {
		env = append(env, "CGO_ENABLED=0")
	}
	for serviceName, cfgString := range b.configs {
		env = append(env, "ENCORE_CFG_"+strings.ToUpper(serviceName)+"="+base64.RawURLEncoding.EncodeToString([]byte(cfgString)))
	}

	cmd.Env = append(os.Environ(), env...)
	cmd.Dir = filepath.Join(b.appRoot, b.cfg.WorkingDir)
	cmd.Stdout = b.cfg.Test.Stdout
	cmd.Stderr = b.cfg.Test.Stderr
	return cmd.Run()
}
