package usage

import (
	"flag"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/rogpeppe/go-internal/txtar"
	"golang.org/x/exp/slices"

	"encr.dev/pkg/fns"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/testutil"
	"encr.dev/v2/parser"
)

var goldenUpdate = flag.Bool("golden-update", false, "update golden files")

func TestParse(t *testing.T) {
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
				if !os.IsNotExist(err) {
					c.Fatal(err)
				}
				modContents := "module example.com\nrequire encore.dev v1.13.4"
				err := os.WriteFile(modPath, []byte(modContents), 0644)
				c.Assert(err, qt.IsNil)
			}

			tc.GoModTidy()
			tc.GoModDownload()

			p := parser.NewParser(tc.Context)
			res := p.Parse()
			usages := Parse(res.Packages, res.InfraBinds)

			got := fns.Map(usages, func(u Usage) usageDesc { return usageToDesc(tc.Context, u) })
			want := test.wants

			// Sort the slices to be able to compare them.
			for _, slice := range [][]usageDesc{got, want} {
				slices.SortFunc(slice, func(a, b usageDesc) bool {
					if a.Filename != b.Filename {
						return a.Filename < b.Filename
					}
					if a.Line != b.Line {
						return a.Line < b.Line
					}
					if a.Resource != b.Resource {
						return a.Resource < b.Resource
					}
					return a.Operation < b.Operation
				})
			}

			if *goldenUpdate {
				//updateGoldenFiles(c, test, got)
			}
			if diff := cmp.Diff(got, want); diff != "" {
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

func usageToDesc(pc *parsectx.Context, u Usage) usageDesc {
	pos := pc.FS.Position(u.ASTExpr().Pos())
	filename := pos.Filename
	if rel, err := filepath.Rel(pc.MainModuleDir.ToIO(), pos.Filename); err == nil {
		filename = rel
	}

	return usageDesc{
		Filename:  filename,
		Line:      pos.Line,
		Resource:  u.ResourceBind().QualifiedName().NaiveDisplayName(),
		Operation: u.operationDesc(),
	}
}
