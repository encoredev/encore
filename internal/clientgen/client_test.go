//go:build go1.18
// +build go1.18

package clientgen

import (
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

func TestClientCodeGeneration(t *testing.T) {
	t.Helper()
	c := qt.New(t)

	tests, err := filepath.Glob("./testdata/input*.go")
	if err != nil {
		t.Fatal(err)
	}
	c.Assert(err, qt.IsNil)

	for _, path := range tests {
		path := path
		c.Run("expected"+strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".go"), "input"), func(c *qt.C) {
			ar, err := txtar.ParseFile(path)
			c.Assert(err, qt.IsNil)

			base := t.TempDir()
			err = txtar.Write(ar, base)
			c.Assert(err, qt.IsNil)

			res, err := parser.Parse(&parser.Config{
				AppRoot:    base,
				ModulePath: "app",
			})
			c.Assert(err, qt.IsNil)

			files, err := os.ReadDir("./testdata")
			c.Assert(err, qt.IsNil)

			expectedPrefix := "expected" + strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".go"), "input") + "_"

			for _, file := range files {
				testName := strings.TrimPrefix(file.Name(), expectedPrefix)

				// Check that the trim prefix removed the expectedPrefix && there are no other underscores in the testName
				if testName != file.Name() && !strings.Contains(testName, "_") {
					c.Run(testName, func(c *qt.C) {
						language, ok := Detect(file.Name())
						c.Assert(ok, qt.IsTrue, qt.Commentf("Unable to detect language type for %s", file.Name()))

						generatedClient, err := Client(language, "app", res.Meta)
						c.Assert(err, qt.IsNil)

						golden.TestAgainst(c, file.Name(), string(generatedClient))
					})
				}
			}
		})
	}
}
