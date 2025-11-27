package codegentest

import (
	stdcmp "cmp"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/rogpeppe/go-internal/renameio"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/v2/app"
	"encr.dev/v2/codegen"
	"encr.dev/v2/compiler/build"
	"encr.dev/v2/internals/overlay"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
)

type Case struct {
	Name     string
	Code     string
	Want     map[string]string // file name -> contents
	WantErrs []string
}

var goldenUpdate = flag.Bool("golden-update", os.Getenv("GOLDEN_UPDATE") != "", "update golden files")

func Run(t *testing.T, fn func(*codegen.Generator, *app.Desc)) {
	flag.Parse()
	c := qt.New(t)
	tests := readTestCases(c, "testdata")
	for _, test := range tests {
		c.Run(test.name, func(c *qt.C) {
			tc := testutil.NewContext(c, false, test.input)
			tc.FailTestOnErrors()

			// Create a go.mod file in the main module directory if it doesn't already exist.
			modPath := tc.MainModuleDir.Join("go.mod").ToIO()
			if _, err := os.Stat(modPath); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					c.Fatal(err)
				}
				modContents := "module example.com\nrequire encore.dev v1.52.0"
				err := os.WriteFile(modPath, []byte(modContents), 0644)
				c.Assert(err, qt.IsNil)
			}

			tc.GoModTidy()
			tc.GoModDownload()

			p := parser.NewParser(tc.Context)
			parserResult := p.Parse()
			gen := codegen.New(tc.Context, nil)
			appDesc := app.ValidateAndDescribe(tc.Context, parserResult)

			// Run the codegen
			fn(gen, appDesc)

			// Construct the map of generated code.
			overlays := gen.Overlays()
			got := make(map[string]string, len(overlays))
			for _, o := range overlays {
				// Try to compute a reasonable display path for the file.
				// If it's local within the main module, use that.
				// Otherwise check if it's within the runtime module,
				// and finally fall back to the absolute path.
				key := o.Source.ToIO()

				mainRel, err := filepath.Rel(tc.MainModuleDir.ToIO(), o.Source.ToIO())
				if err == nil && filepath.IsLocal(mainRel) {
					key = mainRel
				} else {
					runtimeRel, err := filepath.Rel(tc.Build.EncoreRuntime.ToIO(), o.Source.ToIO())
					if err == nil && filepath.IsLocal(runtimeRel) {
						key = runtimeRel
					}
				}

				got[key] = string(o.Contents)
			}

			if *goldenUpdate {
				updateGoldenFiles(c, test, got)
			} else if diff := cmp.Diff(got, test.want); diff != "" {
				c.Fatalf("generated code differs (-got +want):\n%s", diff)
			}

			// Make sure it compiles
			goBuild(tc, overlays)
		})
	}
}

func readTestCases(c *qt.C, dir string) []*testCase {
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	c.Assert(err, qt.IsNil)

	var cases []*testCase
	for _, file := range files {
		cases = append(cases, parseTestCase(c, file))
	}
	return cases
}

type testCase struct {
	filename string
	name     string
	input    *txtar.Archive
	want     map[string]string
}

func parseTestCase(c *qt.C, file string) *testCase {
	ar, err := txtar.ParseFile(file)
	c.Assert(err, qt.IsNil)

	want := make(map[string]string)
	for i := 0; i < len(ar.Files); i++ {
		f := ar.Files[i]

		if fn, ok := strings.CutPrefix(f.Name, "want:"); ok {
			want[fn] = string(f.Data)
			ar.Files = slices.Delete(ar.Files, i, i+1)
			i--
		}
	}

	return &testCase{
		filename: file,
		name:     strings.TrimSuffix(filepath.Base(file), ".txt"),
		input:    ar,
		want:     want,
	}
}

func updateGoldenFiles(c *qt.C, tc *testCase, got map[string]string) {
	var goldenFiles []txtar.File
	for key, val := range got {
		goldenFiles = append(goldenFiles, txtar.File{
			Name: "want:" + key,
			Data: []byte(val),
		})
	}

	slices.SortFunc(goldenFiles, func(a, b txtar.File) int {
		return stdcmp.Compare(a.Name, b.Name)
	})

	tc.input.Files = append(tc.input.Files, goldenFiles...)
	err := renameio.WriteFile(tc.filename, txtar.Format(tc.input))
	c.Assert(err, qt.IsNil)
}

func goBuild(tc *testutil.Context, overlays []overlay.File) {
	build.Build(tc.Context.Ctx, &build.Config{
		Ctx:      tc.Context,
		Overlays: overlays,
		MainPkg:  "./...",
		NoBinary: true,
	})
}
