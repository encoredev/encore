package cuegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser"
	"encr.dev/pkg/golden"
)

func TestMain(m *testing.M) {
	golden.TestMain(m)
}

func TestCodeGen_TestMain(t *testing.T) {
	c := qt.New(t)
	tests, err := filepath.Glob("./testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	c.Assert(err, qt.IsNil)

	for i, test := range tests {
		path := test
		name := strings.TrimSuffix(filepath.Base(test), ".txt")
		c.Run(name, func(c *qt.C) {
			archiveData, err := os.ReadFile(path)
			c.Assert(err, qt.IsNil)
			a := txtar.Parse(archiveData)
			base := c.TempDir()
			err = txtar.Write(a, base)
			c.Assert(err, qt.IsNil, qt.Commentf("test #%d", i))

			res, err := parser.Parse(&parser.Config{
				AppRoot:    base,
				ModulePath: "encore.app",
				WorkingDir: ".",
			})
			c.Assert(err, qt.IsNil)

			gen := NewGenerator(res)

			for _, svc := range res.App.Services {
				f, err := gen.UserFacing(svc)
				c.Assert(err, qt.IsNil)

				golden.TestAgainst(c.TB, fmt.Sprintf("%s_%s.cue", name, svc.Name), string(f))
			}
		})
	}
}
