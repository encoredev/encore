package codegen_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/rs/zerolog"

	"encr.dev/pkg/paths"
	"encr.dev/v2/codegen"
	"encr.dev/v2/internals/overlay"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
)

// TestOverlaysRendersDeterministically checks that rendering the generated
// files in parallel gives the same names and bytes every run. Two of the files
// share a base name in directories that share theirs, so overlay.Write gives
// them the same name candidate and the second needs a suffix in slice order.
func TestOverlaysRendersDeterministically(t *testing.T) {
	dirs := []paths.FS{
		"/app/a/types",
		"/app/b/types",
		"/app/svc",
	}

	newGen := func() *codegen.Generator {
		ctx := context.Background()
		fs := token.NewFileSet()
		c := &parsectx.Context{
			Ctx:  ctx,
			Log:  zerolog.Nop(),
			FS:   fs,
			Errs: perr.NewList(ctx, fs),
		}
		gen := codegen.New(c, nil)
		for i, dir := range dirs {
			f := gen.InjectFile(paths.Pkg(fmt.Sprintf("example.com/app%d", i)), "types", dir, "encore_internal__api.go", "api")
			f.VarDecl("hello").Value(jen.Lit(strconv.Itoa(i)))
		}
		return gen
	}

	// write renders the overlays to a fresh directory and returns the name and
	// contents hash of the file each source path was written to.
	write := func(t *testing.T, overlays []overlay.File) map[string]string {
		t.Helper()
		overlayFile, err := overlay.Write(paths.FS(t.TempDir()), overlays)
		if err != nil {
			t.Fatalf("overlay.Write: %v", err)
		}
		data, err := os.ReadFile(overlayFile.ToIO())
		if err != nil {
			t.Fatalf("read overlay.json: %v", err)
		}
		var manifest struct{ Replace map[string]string }
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("parse overlay.json: %v", err)
		}

		written := make(map[string]string, len(manifest.Replace))
		for src, dst := range manifest.Replace {
			contents, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read %s: %v", dst, err)
			}
			written[src] = fmt.Sprintf("%s:%x", filepath.Base(dst), sha256.Sum256(contents))
		}
		return written
	}

	var want map[string]string
	for i := range 5 {
		overlays := newGen().Overlays()

		if !slices.IsSortedFunc(overlays, func(a, b overlay.File) int {
			return strings.Compare(a.Source.ToIO(), b.Source.ToIO())
		}) {
			t.Fatalf("run %d: overlays are not sorted by source: %v", i, overlays)
		}

		got := write(t, overlays)
		if i == 0 {
			want = got
			if !slices.ContainsFunc(slices.Collect(maps.Values(want)), func(v string) bool {
				return strings.HasPrefix(v, "gen_types__encore_internal__api_1.go:")
			}) {
				t.Fatalf("expected one of the clashing files to get a _1 suffix, got %v", want)
			}
			continue
		}
		if !maps.Equal(got, want) {
			t.Fatalf("run %d: generated files differ:\ngot  %v\nwant %v", i, got, want)
		}
	}
}

// TestOverlaysReportsRenderPanic checks that a declaration that makes jennifer panic is
// reported as a build error and does not stop the other files rendering.
func TestOverlaysReportsRenderPanic(t *testing.T) {
	ctx := context.Background()
	fs := token.NewFileSet()
	errs := perr.NewList(ctx, fs)
	gen := codegen.New(&parsectx.Context{
		Ctx:  ctx,
		Log:  zerolog.Nop(),
		FS:   fs,
		Errs: errs,
	}, nil)

	good := gen.InjectFile("example.com/good", "good", "/app/good", "encore_internal__api.go", "api")
	good.VarDecl("hello").Value(jen.Lit("hi"))
	bad := gen.InjectFile("example.com/bad", "bad", "/app/bad", "encore_internal__api.go", "api")
	bad.VarDecl("hello").Value(jen.Lit(struct{}{}))

	overlays := gen.Overlays()
	if len(overlays) != 1 || overlays[0].Source != paths.FS("/app/good").Join("encore_internal__api.go") {
		t.Fatalf("expected only the good file to render, got %v", overlays)
	}
	if errs.Len() != 1 {
		t.Fatalf("expected one error, got %d", errs.Len())
	}
}
