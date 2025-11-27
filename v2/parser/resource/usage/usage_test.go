package usage_test

import (
	"cmp"
	"flag"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
	"encr.dev/v2/parser/resource/usage"
)

var goldenUpdate = flag.Bool("golden-update", os.Getenv("GOLDEN_UPDATE") != "", "update golden files")

func TestParse(t *testing.T) {
	flag.Parse()
	c := qt.New(t)
	tests := readTestCases(c, "testdata")
	for _, test := range tests {
		c.Run(test.name, func(c *qt.C) {
			tc := testutil.NewContext(c, false, test.input)
			tc.FailTestOnErrors()
			defer tc.FailTestOnBailout()

			// Create a go.mod file in the main module directory if it doesn't already exist.
			modPath := tc.MainModuleDir.Join("go.mod").ToIO()
			if _, err := os.Stat(modPath); err != nil {
				if !os.IsNotExist(err) {
					c.Fatal(err)
				}
				modContents := "module example.com\nrequire encore.dev v1.52.0"
				err := os.WriteFile(modPath, []byte(modContents), 0644)
				c.Assert(err, qt.IsNil)
			}

			tc.GoModTidy()
			tc.GoModDownload()

			p := parser.NewParser(tc.Context)
			res := p.Parse()

			got := fns.Map(res.AllUsageExprs(), func(u usage.Expr) usageDesc {
				return usageToDesc(tc.Context, u)
			})
			want := test.wants

			// Sort the slices to be able to compare them.
			for _, slice := range [][]usageDesc{got, want} {
				slices.SortFunc(slice, func(a, b usageDesc) int {
					if n := cmp.Compare(a.Filename, b.Filename); n != 0 {
						return n
					} else if n := cmp.Compare(a.Line, b.Line); n != 0 {
						return n
					} else if n := cmp.Compare(a.Resource, b.Resource); n != 0 {
						return n
					}
					return cmp.Compare(a.Operation, b.Operation)
				})
			}

			if *goldenUpdate {
				//updateGoldenFiles(c, test, got)
			}
			if diff := gocmp.Diff(got, want); diff != "" {
				c.Fatalf("generated code differs (-got +want):\n%s", diff)
			}
		})
	}
}

type testCase struct {
	filename string
	name     string
	input    *txtar.Archive
	wants    []usageDesc
}

type usageDesc struct {
	Filename  string
	Line      int
	Resource  string
	Operation string
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

func parseTestCase(c *qt.C, file string) *testCase {
	ar, err := txtar.ParseFile(file)
	c.Assert(err, qt.IsNil)

	tc := &testCase{
		filename: file,
		name:     strings.TrimSuffix(filepath.Base(file), ".txt"),
		input:    ar,
	}

	for _, f := range ar.Files {
		if strings.HasSuffix(f.Name, ".go") {
			fset := gotoken.NewFileSet()
			astFile, err := goparser.ParseFile(fset, f.Name, f.Data, goparser.ParseComments)
			c.Assert(err, qt.IsNil)

			for _, cg := range astFile.Comments {
				for i, comment := range cg.List {
					if remainder, ok := strings.CutPrefix(comment.Text, "// use "); ok {
						resource, op, _ := strings.Cut(remainder, " ")
						tc.wants = append(tc.wants, usageDesc{
							Filename:  f.Name,
							Line:      fset.Position(cg.Pos()).Line + i,
							Resource:  resource,
							Operation: op,
						})
					}
				}
			}

		}
	}

	return tc
}

func usageToDesc(pc *parsectx.Context, u usage.Expr) usageDesc {
	pos := pc.FS.Position(u.ASTExpr().Pos())
	filename := pos.Filename
	if rel, err := filepath.Rel(pc.MainModuleDir.ToIO(), pos.Filename); err == nil {
		filename = rel
	}

	return usageDesc{
		Filename:  filename,
		Line:      pos.Line,
		Resource:  u.ResourceBind().DescriptionForTest(),
		Operation: u.DescriptionForTest(),
	}
}
