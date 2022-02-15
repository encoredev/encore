//go:build go1.18
// +build go1.18

package codegen

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/parser"
)

func TestClientCodeGeneration(t *testing.T) {
	t.Helper()
	c := qt.New(t)

	ar, err := txtar.ParseFile("testdata/input.go")
	c.Assert(err, qt.IsNil)

	base := t.TempDir()
	err = txtar.Write(ar, base)
	c.Assert(err, qt.IsNil)

	res, err := parser.Parse(&parser.Config{
		AppRoot:    base,
		ModulePath: "app",
	})
	c.Assert(err, qt.IsNil)

	files, err := ioutil.ReadDir("./testdata")
	c.Assert(err, qt.IsNil)

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "expected_") {
			c.Run(file.Name()[9:], func(c *qt.C) {
				language, ok := Detect(file.Name())
				c.Assert(ok, qt.IsTrue, qt.Commentf("Unable to detect language type for %s", file.Name()))

				ts, err := Client(language, "app", res.Meta)
				c.Assert(err, qt.IsNil)

				expect, err := ioutil.ReadFile(filepath.Join("testdata", file.Name()))
				c.Assert(err, qt.IsNil)

				c.Assert(strings.Split(string(ts), "\n"), qt.DeepEquals, strings.Split(string(expect), "\n"))
			})
		}
	}
}
