package compiler_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	ts "github.com/rogpeppe/go-internal/testscript"

	"encr.dev/compiler"
)

func TestCompile(t *testing.T) {
	runtimePath := os.Getenv("ENCORE_RUNTIME_PATH")
	goroot := os.Getenv("ENCORE_GOROOT")
	if testing.Short() {
		t.Skip("skipping in short mode")
	} else if runtimePath == "" || goroot == "" {
		t.Skipf("skipping due to missing ENCORE_RUNTIME_PATH=%q or ENCORE_GOROOT=%q", runtimePath, goroot)
	}

	home := t.TempDir()

	ts.Run(t, ts.Params{
		Dir: "testdata",
		Setup: func(e *ts.Env) error {
			e.Setenv("ENCORE_RUNTIME_PATH", runtimePath)
			e.Setenv("ENCORE_GOROOT", goroot)
			e.Setenv("HOME", home)
			e.Setenv("GOFLAGS", "-modcacherw")
			gomod := []byte("module test\n\nrequire encore.dev v0.17.0")
			return ioutil.WriteFile(filepath.Join(e.WorkDir, "go.mod"), gomod, 0755)
		},
	})
}

func TestMain(m *testing.M) {
	os.Exit(ts.RunMain(m, map[string]func() int{
		"build": func() int {
			wd, err := os.Getwd()
			if err != nil {
				os.Stderr.WriteString(err.Error())
				return 1
			}
			cfg := &compiler.Config{
				WorkingDir:        ".",
				EncoreGoRoot:      os.Getenv("ENCORE_GOROOT"),
				EncoreRuntimePath: os.Getenv("ENCORE_RUNTIME_PATH"),
				BuildTags:         []string{"encore_local"},
			}
			if _, err := compiler.Build(wd, cfg); err != nil {
				os.Stderr.WriteString(err.Error())
				return 1
			}
			return 0
		},
	}))
}
