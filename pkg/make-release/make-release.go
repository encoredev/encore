package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Builder struct {
	GOOS     string
	GOARCH   string
	encoreGo string
	dst      string
	version  string
}

func (b *Builder) PrepareWorkdir() error {
	if err := os.RemoveAll(b.dst); err != nil {
		return err
	} else if err := os.MkdirAll(b.dst, 0755); err != nil {
		return err
	} else if err := os.MkdirAll(join(b.dst, "bin"), 0755); err != nil {
		return err
	}
	return nil
}

func (b *Builder) BuildDashFrontend() error {
	dir := join("cli", "daemon", "dash", "dashapp")
	npmInst := exec.Command("npm", "install")
	npmInst.Dir = dir
	if out, err := npmInst.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install failed: %s (%v)", out, err)
	}

	npmBuild := exec.Command("npm", "run", "build")
	npmBuild.Dir = dir
	if out, err := npmBuild.CombinedOutput(); err != nil {
		return fmt.Errorf("npm run build failed: %s (%v)", out, err)
	}
	return nil
}

func (b *Builder) BuildBinaries() error {
	env := append(os.Environ(),
		"GOOS="+b.GOOS,
		"GOARCH="+b.GOARCH,
		"CGO_ENABLED=1",
	)

	switch {
	case b.GOOS == "darwin":
		// Darwin needs to specify the target when cross-compiling.
		var target string
		switch b.GOARCH {
		case "amd64":
			target = "x86_64-apple-macos10.12"
		case "arm64":
			target = "arm64-apple-macos11"
			env = append(env, "SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX11.sdk")
		default:
			return fmt.Errorf("unsupported GOARCH %q", b.GOARCH)
		}
		env = append(env,
			"LDFLAGS=--target="+target,
			"CFLAGS=-O3 --target="+target,
		)

	case b.GOOS == "linux" && b.GOARCH == "arm64":
		// GitHub Actions doesn't have builders for linux/arm64 so we need to
		// cross-compile. Unfortunately we need cgo for sqlite so use zig to do so.
		mustWriteFile("/usr/local/bin/zcc", 0755,
			"#!/bin/sh\nzig cc -Wl,--no-gc-sections -target aarch64-linux-gnu.2.28 $@")
		mustWriteFile("/usr/local/bin/zxx", 0755,
			"#!/bin/sh\nzig c++ -Wl,--no-gc-sections -target aarch64-linux-gnu.2.28 $@")
		env = append(env,
			"CC=/usr/local/bin/zcc",
			"CXX=/usr/local/bin/zxx",
		)
	}

	// Nightly builds don't prefix with "v"
	version := b.version
	if !strings.HasPrefix(version, "nightly-") {
		version = "v" + version
	}

	cmd := exec.Command("go", "build",
		fmt.Sprintf("-ldflags=-X 'encr.dev/internal/version.Version=%s'", version),
		"-o", join(b.dst, "bin", "encore"),
		"./cli/cmd/encore")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build encore failed: %s (%v)", out, err)
	}

	cmd = exec.Command("go", "build",
		"-o", join(b.dst, "bin", "git-remote-encore"),
		"./cli/cmd/git-remote-encore")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build git-remote-encore failed: %s (%v)", out, err)
	}

	return nil
}

func (b *Builder) CopyEncoreGo() error {
	cmd := exec.Command("cp", "-r", b.encoreGo, join(b.dst, "encore-go"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp %v failed: %s (%v)", cmd.Args, out, err)
	}
	return nil
}

func (b *Builder) CopyRuntime() error {
	cmd := exec.Command("cp", "-r", "runtime", join(b.dst, "runtime"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cp %v failed: %s (%v)", cmd.Args, out, err)
	}
	return nil
}

func join(strs ...string) string {
	return filepath.Join(strs...)
}

func all(src string, all ...string) []string {
	var res []string
	for _, a := range all {
		res = append(res, join(src, a))
	}
	return res
}

func main() {
	goos := flag.String("goos", "", "GOOS")
	goarch := flag.String("goarch", "", "GOARCH")
	dst := flag.String("dst", "", "build destination")
	version := flag.String("v", "", "version number (without 'v')")
	encoreGo := flag.String("encore-go", "", "path to encore-go root")
	flag.Parse()
	if *goos == "" || *goarch == "" || *dst == "" || *version == "" || *encoreGo == "" {
		log.Fatalf("missing -dst %q, -goos %q, -goarch %q, -v %q, or -encore-go %q", *dst, *goos, *goarch, *version, *encoreGo)
	}

	if *goos == "windows" {
		log.Fatalf("cannot use make-release.go for Windows builds. use ./windows/build.bat instead.")
	}

	root, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	} else if _, err := os.Stat(join(root, "go.mod")); err != nil {
		log.Fatalln("expected to run make-release.go from encr.dev repository root")
	}

	*dst, err = filepath.Abs(*dst)
	if err != nil {
		log.Fatalln(err)
	}

	b := &Builder{
		GOOS:     *goos,
		GOARCH:   *goarch,
		encoreGo: filepath.FromSlash(*encoreGo),
		dst:      join(*dst, *goos+"_"+*goarch),
		version:  *version,
	}

	for _, f := range []func() error{
		b.PrepareWorkdir,
		b.BuildDashFrontend,
		b.BuildBinaries,
		b.CopyEncoreGo,
		b.CopyRuntime,
	} {
		if err := f(); err != nil {
			log.Fatalln(err)
		}
	}
}

func mustWriteFile(filename string, perm fs.FileMode, contents string) {
	if err := os.WriteFile(filename, []byte(contents), perm); err != nil {
		log.Fatalln(err)
	}
}
