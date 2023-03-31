package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/overlay"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"run": run,
	}))
}

func TestBuild(t *testing.T) {
	// Get existing go cache, if any
	gocache, err := exec.Command("go", "env", "GOCACHE").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	gocache = bytes.TrimSpace(gocache)
	if len(gocache) == 0 {
		gocache = []byte(t.TempDir())
	}

	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			env.Setenv(testutil.EnvRepoDirOverride, testutil.EncoreRepoDir)
			env.Setenv("GOCACHE", string(gocache))
			fsPath := paths.RootedFSPath(env.WorkDir, ".")
			overlays, err := processOverlays(fsPath)
			if err != nil {
				return err
			}
			jsonOverlays, _ := json.Marshal(overlays)
			env.Setenv("CODEGEN_OVERLAYS", string(jsonOverlays))
			return nil
		},
	})
}

func run() int {
	if len(os.Args) != 2 {
		log.Fatal("usage: run <pkg>")
	}

	var overlays []overlay.File
	if desc := os.Getenv("CODEGEN_OVERLAYS"); desc != "" {
		if err := json.Unmarshal([]byte(desc), &overlays); err != nil {
			log.Fatalf("failed to unmarshal CODEGEN_OVERLAYS: %v", err)
		}
	}

	work := os.Getenv("WORK")
	res := build(work, paths.MustPkgPath(os.Args[1]), overlays)
	cmd := exec.Command(res.Exe.ToIO())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func build(workdir string, pkgPath paths.Pkg, overlays []overlay.File) *Result {
	runtimeArchive := testutil.ParseTxtar(dummyEncoreRuntime)
	if err := txtar.Write(runtimeArchive, workdir); err != nil {
		log.Fatalf("failed to write runtime archive: %v", err)
	}
	wd := paths.RootedFSPath(workdir, ".")

	ctx := context.Background()
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	pc := &parsectx.Context{
		Ctx: ctx,
		Log: zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.InfoLevel),
		Build: parsectx.BuildInfo{
			Experiments: nil,
			GOARCH:      runtime.GOARCH,
			GOOS:        runtime.GOOS,
			GOROOT:      paths.RootedFSPath(runtime.GOROOT(), "."),
			// HACK(andre): Make this nicer
			EncoreRuntime: wd.Join("encore-runtime"),
			BuildTags:     nil,
			CgoEnabled:    false,
			StaticLink:    false,
			Debug:         false,
		},
		FS:            fs,
		ParseTests:    false,
		Errs:          errs,
		MainModuleDir: wd,
	}

	res := Build(ctx, &Config{
		Ctx:        pc,
		Overlays:   overlays,
		MainPkg:    pkgPath,
		KeepOutput: false,
	})
	if errs.Len() > 0 {
		log.Fatalf("build failed: %s", errs.FormatErrors())
	}
	return res
}

const dummyEncoreRuntime = `
-- encore-runtime/go.mod --
module encore.dev
-- encore-runtime/go.sum --
`

func processOverlays(workdir paths.FS) ([]overlay.File, error) {
	var overlays []overlay.File

	files, _ := os.ReadDir(workdir.ToIO())
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		name := f.Name()
		overlayPath, ok := strings.CutPrefix(name, "overlay:")
		if !ok {
			continue
		}
		overlayPath = strings.ReplaceAll(overlayPath, "\\", "/")

		src := workdir.Join(filepath.FromSlash(overlayPath))
		dst := workdir.Join(name)

		contents, err := os.ReadFile(dst.ToIO())
		if err != nil {
			log.Fatal(err)
		}
		overlays = append(overlays, overlay.File{
			Source:   src,
			Contents: contents,
		})

		if err := os.Remove(dst.ToIO()); err != nil {
			return nil, fmt.Errorf("unable to delete file: %v", err)
		}
	}

	return overlays, nil
}
