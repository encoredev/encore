package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
		"CGO_ENABLED=1",
		"GOOS="+b.GOOS,
		"GOARCH="+b.GOARCH,
	)

	if b.GOOS == "darwin" {
		var target string
		switch b.GOARCH {
		case "amd64":
			target = "x86_64-apple-macos10.12"
		case "arm64":
			target = "arm64-apple-macos11"
			env = append(env, "SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX11.1.sdk")
		default:
			return fmt.Errorf("unsupported GOARCH %q", b.GOARCH)
		}
		env = append(env,
			"LDFLAGS=--target="+target,
			"CFLAGS=-O3 --target="+target,
		)
	}

	cmd := exec.Command("go", "build",
		fmt.Sprintf("-ldflags=-X 'encr.dev/cli/internal/version.Version=v%s'", b.version),
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
	cmd := exec.Command("cp", "-r", join("compiler", "runtime"), join(b.dst, "runtime"))
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
