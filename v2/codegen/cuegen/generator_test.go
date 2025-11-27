package cuegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/golden"
	"encr.dev/v2/app"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
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

	for _, test := range tests {
		path := test
		name := strings.TrimSuffix(filepath.Base(test), ".txt")
		c.Run(name, func(c *qt.C) {
			archiveData, err := os.ReadFile(path)
			c.Assert(err, qt.IsNil)
			a := txtar.Parse(archiveData)

			a.Files = append(a.Files, txtar.File{
				Name: "go.mod",
				Data: []byte("module encore.app\nrequire encore.dev v1.52.0\n"),
			})

			tc := testutil.NewContext(c, false, a)
			tc.GoModDownload()

			tc.FailTestOnErrors()
			defer tc.FailTestOnBailout()

			result := parser.NewParser(tc.Context).Parse()
			desc := app.ValidateAndDescribe(tc.Context, result)
			gen := NewGenerator(desc)

			for _, svc := range desc.Services {
				f, err := gen.UserFacing(svc)
				c.Assert(err, qt.IsNil)

				golden.TestAgainst(c.TB, fmt.Sprintf("%s_%s.cue", name, svc.Name), string(f))
			}
		})
	}
}
