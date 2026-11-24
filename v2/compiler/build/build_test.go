package build

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	builderpkg "encr.dev/pkg/builder"
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

func TestBuildDebugFlags(t *testing.T) {
	tests := []struct {
		name        string
		debug       builderpkg.DebugMode
		omit        bool
		wantGCFlags string
		wantStrip   bool
	}{
		{name: "disabled", debug: builderpkg.DebugModeDisabled},
		{name: "disabled_omit", debug: builderpkg.DebugModeDisabled, omit: true, wantStrip: true},
		{name: "enabled_omit", debug: builderpkg.DebugModeEnabled, omit: true, wantGCFlags: "all=-N -l"},
		{name: "break_omit", debug: builderpkg.DebugModeBreak, omit: true, wantGCFlags: "all=-N -l"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := buildEmptyMain(t, &Config{StaticConfig: &config.Static{}, OmitDebugInfo: tt.omit}, tt.debug)
			if got := settings["-gcflags"]; got != tt.wantGCFlags {
				t.Errorf("-gcflags = %q, want %q", got, tt.wantGCFlags)
			}
			ldflags := strings.Fields(settings["-ldflags"])
			if got := slices.Contains(ldflags, "-s") && slices.Contains(ldflags, "-w"); got != tt.wantStrip {
				t.Errorf("-ldflags %q has -s -w: %v, want %v", settings["-ldflags"], got, tt.wantStrip)
			}
		})
	}
}

// buildEmptyMain builds a one-file main package with cfg and returns the binary's build settings.
func buildEmptyMain(t *testing.T, cfg *Config, debug builderpkg.DebugMode) map[string]string {
	t.Helper()
	dir := t.TempDir()
	ar := testutil.ParseTxtar("-- go.mod --\nmodule example.com\n\ngo 1.22\n-- main.go --\npackage main\n\nfunc main() {}\n" + dummyEncoreRuntime)
	if err := txtar.Write(ar, dir); err != nil {
		t.Fatal(err)
	}
	wd := paths.RootedFSPath(dir, ".")
	ctx := context.Background()
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	cfg.Ctx = &parsectx.Context{
		Ctx: ctx,
		Log: zerolog.Nop(),
		Build: parsectx.BuildInfo{
			GOARCH:        runtime.GOARCH,
			GOOS:          runtime.GOOS,
			GOROOT:        paths.RootedFSPath(runtime.GOROOT(), "."),
			EncoreRuntime: wd.Join("encore-runtime"),
			Debug:         debug,
		},
		FS:            fs,
		Errs:          errs,
		MainModuleDir: wd,
	}
	cfg.MainPkg = "example.com"
	res := Build(ctx, cfg)
	if errs.Len() > 0 {
		t.Fatalf("build failed: %s", errs.FormatErrors())
	}

	info, err := buildinfo.ReadFile(res.Exe.ToIO())
	if err != nil {
		t.Fatalf("read build info: %v", err)
	}
	settings := make(map[string]string)
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	return settings
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
			Debug:         builderpkg.DebugModeDisabled,
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
