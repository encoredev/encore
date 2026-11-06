package build

import (
	"context"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/modfile"

	"encr.dev/internal/version"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/overlay"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
)

// goRoot reports the GOROOT of the go command on PATH.
func goRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// newTidyTestBuilder creates an app with a user file and a generated overlay file,
// a dummy Encore runtime, and a persistent workdir.
func newTidyTestBuilder(t *testing.T) *builder {
	t.Helper()
	root := t.TempDir()
	if d, err := filepath.EvalSymlinks(root); err == nil {
		root = d
	}
	appDir := filepath.Join(root, "app")
	writeTestFile(t, filepath.Join(appDir, "go.mod"), "module example.com/app\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(appDir, "svc", "svc.go"), "package svc\n\nimport _ \"fmt\"\n")
	writeTestFile(t, filepath.Join(root, "runtime", "go.mod"), "module encore.dev\n")
	writeTestFile(t, filepath.Join(root, "runtime", "go.sum"), "")
	workdir := filepath.Join(root, "work")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	appFS := paths.RootedFSPath(appDir, ".")
	return &builder{
		ctx:  ctx,
		mode: buildMode,
		errs: perr.NewList(ctx, token.NewFileSet()),
		cfg: &Config{
			Ctx: &parsectx.Context{
				Ctx:   ctx,
				AppID: option.Some("tidy-test"),
				Build: parsectx.BuildInfo{
					GOARCH:        runtime.GOARCH,
					GOOS:          runtime.GOOS,
					GOROOT:        paths.RootedFSPath(goRoot(t), "."),
					EncoreRuntime: paths.RootedFSPath(root, "runtime"),
				},
				MainModuleDir: appFS,
			},
			Overlays: []overlay.File{{
				Source:   appFS.Join("svc", "encore.gen.go"),
				Contents: []byte("package svc\n\nimport _ \"errors\"\n"),
			}},
		},
		workdir: paths.RootedFSPath(workdir, "."),
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// tidyInputs hashes the inputs for the app, using its go.mod as the pre-tidy go.mod.
func tidyInputs(t *testing.T, b *builder) (inputs string, canFingerprint bool) {
	t.Helper()
	modData, err := os.ReadFile(b.cfg.Ctx.MainModuleDir.Join("go.mod").ToIO())
	if err != nil {
		t.Fatal(err)
	}
	mainMod, err := modfile.Parse("go.mod", modData, nil)
	if err != nil {
		t.Fatal(err)
	}
	return b.tidyInputsHash(mainMod, modData)
}

// tidyVerdict reports whether tidy would be skipped.
func tidyVerdict(t *testing.T, b *builder) (skip, canFingerprint bool) {
	t.Helper()
	inputs, ok := tidyInputs(t, b)
	if !ok {
		return false, false
	}
	return b.tidyUpToDate(inputs), true
}

// recordTidy simulates a successful tidy: it writes outputs and the fingerprint.
func recordTidy(t *testing.T, b *builder) {
	t.Helper()
	writeTestFile(t, b.workdir.Join("go.mod").ToIO(), "module example.com/app\n\ngo 1.21\n")
	writeTestFile(t, b.workdir.Join("go.sum").ToIO(), "")
	inputs, ok := tidyInputs(t, b)
	if !ok {
		t.Fatal("tidyInputsHash: cannot fingerprint")
	}
	b.writeTidyFingerprint(inputs)
}

func TestTidyFingerprint(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, b *builder)
	}{
		{
			name: "app go.mod",
			change: func(t *testing.T, b *builder) {
				writeTestFile(t, b.cfg.Ctx.MainModuleDir.Join("go.mod").ToIO(), "module example.com/app\n\ngo 1.22\n")
			},
		},
		{
			name: "import in user file",
			change: func(t *testing.T, b *builder) {
				writeTestFile(t, b.cfg.Ctx.MainModuleDir.Join("svc", "svc.go").ToIO(), "package svc\n\nimport _ \"fmt\"\nimport _ \"golang.org/x/sync/errgroup\"\n")
			},
		},
		{
			name: "import in generated file",
			change: func(t *testing.T, b *builder) {
				b.cfg.Overlays[0].Contents = []byte("package svc\n\nimport _ \"github.com/rs/zerolog\"\n")
			},
		},
		{
			name: "build constraint on a user file",
			change: func(t *testing.T, b *builder) {
				path := b.cfg.Ctx.MainModuleDir.Join("svc", "svc.go").ToIO()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, path, "//go:build ignore\n\n"+string(data))
			},
		},
		{
			name: "build constraint after a block comment",
			change: func(t *testing.T, b *builder) {
				writeTestFile(t, b.cfg.Ctx.MainModuleDir.Join("svc", "svc.go").ToIO(),
					"/* package docs */\n//go:build ignore\n\npackage svc\n\nimport _ \"fmt\"\n")
			},
		},
		{
			name: "go.work above the app",
			change: func(t *testing.T, b *builder) {
				writeTestFile(t, filepath.Join(filepath.Dir(b.cfg.Ctx.MainModuleDir.ToIO()), "go.work"), "go 1.21\n\nuse ./app\n")
			},
		},
		{
			name: "user file renamed with the same imports",
			change: func(t *testing.T, b *builder) {
				dir := b.cfg.Ctx.MainModuleDir.Join("svc").ToIO()
				if err := os.Rename(filepath.Join(dir, "svc.go"), filepath.Join(dir, "_svc.go")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "Encore version",
			change: func(t *testing.T, b *builder) {
				old := version.Version
				t.Cleanup(func() { version.Version = old })
				version.Version = old + "-other"
			},
		},
		{
			name: "GO environment variable",
			change: func(t *testing.T, b *builder) {
				t.Setenv("GOFLAGS", "-mod=mod")
			},
		},
		{
			name: "hand-edited build go.mod",
			change: func(t *testing.T, b *builder) {
				writeTestFile(t, b.workdir.Join("go.mod").ToIO(), "module example.com/app\n\ngo 1.21\n\nrequire example.com/old v1.0.0\n")
			},
		},
		{
			name: "missing build go.sum",
			change: func(t *testing.T, b *builder) {
				if err := os.Remove(b.workdir.Join("go.sum").ToIO()); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "corrupted fingerprint file",
			change: func(t *testing.T, b *builder) {
				writeTestFile(t, b.workdir.Join(tidyFingerprintName).ToIO(), "{not json")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newTidyTestBuilder(t)
			recordTidy(t, b)
			if skip, _ := tidyVerdict(t, b); !skip {
				t.Fatal("got run before the change, want skip")
			}

			tt.change(t, b)
			if skip, _ := tidyVerdict(t, b); skip {
				t.Fatal("got skip after the change, want run")
			}
		})
	}
}

func TestTidyFingerprint_LocalReplaceAlwaysRuns(t *testing.T) {
	b := newTidyTestBuilder(t)
	recordTidy(t, b)
	writeTestFile(t, b.cfg.Ctx.MainModuleDir.Join("go.mod").ToIO(),
		"module example.com/app\n\ngo 1.21\n\nreplace example.com/lib => ../lib\n")
	if _, canFingerprint := tidyVerdict(t, b); canFingerprint {
		t.Fatal("got a fingerprint with a local replace, want tidy to always run")
	}
}

func TestTidyFingerprint_TemporaryWorkdirAlwaysRuns(t *testing.T) {
	b := newTidyTestBuilder(t)
	b.cfg.Ctx.AppID = option.None[string]()
	if _, canFingerprint := tidyVerdict(t, b); canFingerprint {
		t.Fatal("got a fingerprint for a temporary workdir, want tidy to always run")
	}
}

// TestWriteModFile_SkipsTidy runs the real 'go mod tidy' through writeModFile.
func TestWriteModFile_SkipsTidy(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	b := newTidyTestBuilder(t)
	gomod := b.workdir.Join("go.mod").ToIO()
	fingerprint := b.workdir.Join(tidyFingerprintName).ToIO()

	// ranTidy backdates go.mod, so a build that runs tidy and rewrites it shows a newer mtime.
	ranTidy := func() bool {
		t.Helper()
		old := time.Now().Add(-time.Hour).Truncate(time.Second)
		if err := os.Chtimes(gomod, old, old); err != nil {
			t.Fatal(err)
		}
		b.writeModFile()
		if b.errs.Len() > 0 {
			t.Fatalf("writeModFile: %s", b.errs.FormatErrors())
		}
		if _, err := os.Stat(fingerprint); err != nil {
			t.Fatalf("fingerprint file missing after build: %v", err)
		}
		fi, err := os.Stat(gomod)
		if err != nil {
			t.Fatal(err)
		}
		return !fi.ModTime().Equal(old)
	}

	// The app has no dependencies, so tidy keeps but never creates go.sum.
	writeTestFile(t, b.workdir.Join("go.sum").ToIO(), "")
	b.writeModFile()
	if b.errs.Len() > 0 {
		t.Fatalf("first writeModFile: %s", b.errs.FormatErrors())
	}
	if ranTidy() {
		t.Fatal("second build with no change ran tidy, want skip")
	}

	writeTestFile(t, b.cfg.Ctx.MainModuleDir.Join("svc", "svc.go").ToIO(), "package svc\n\nimport _ \"fmt\"\nimport _ \"strings\"\n")
	if !ranTidy() {
		t.Fatal("build after adding an import skipped tidy, want run")
	}
	if ranTidy() {
		t.Fatal("build after the re-tidy ran tidy again, want skip")
	}
}

func TestTidyFingerprint_BodyEditSkips(t *testing.T) {
	b := newTidyTestBuilder(t)
	recordTidy(t, b)
	writeTestFile(t, b.cfg.Ctx.MainModuleDir.Join("svc", "svc.go").ToIO(),
		"package svc\n\nimport _ \"fmt\"\n\nfunc Hello() string { return \"hi\" }\n")
	if skip, _ := tidyVerdict(t, b); !skip {
		t.Fatal("got run after an edit with the same imports, want skip")
	}
}
