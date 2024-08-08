package clientgen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/cli/daemon/apps"
	"encr.dev/internal/clientgen/clientgentypes"
	"encr.dev/pkg/builder"
	"encr.dev/pkg/golden"
	"encr.dev/v2/tsbuilder"
	"encr.dev/v2/v2builder"
)

func TestMain(m *testing.M) {
	golden.TestMain(m)
}

func TestClientCodeGenerationFromGoApp(t *testing.T) {
	t.Helper()
	c := qt.New(t)

	tests, err := filepath.Glob("./testdata/goapp/input*.go")
	if err != nil {
		t.Fatal(err)
	}
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	bld := v2builder.BuilderImpl{}

	for _, path := range tests {
		path := path
		c.Run("expected"+strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".go"), "input"), func(c *qt.C) {
			ar, err := txtar.ParseFile(path)
			c.Assert(err, qt.IsNil)

			base := t.TempDir()
			err = txtar.Write(ar, base)
			c.Assert(err, qt.IsNil)

			res, err := bld.Parse(ctx, builder.ParseParams{
				Build:       builder.DefaultBuildInfo(),
				App:         apps.NewInstance(base, "app", ""),
				Experiments: nil,
				WorkingDir:  ".",
				ParseTests:  false,
			})
			c.Assert(err, qt.IsNil)

			files, err := os.ReadDir("./testdata/goapp")
			c.Assert(err, qt.IsNil)

			expectedPrefix := "expected" + strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".go"), "input") + "_"

			for _, file := range files {
				testName := strings.TrimPrefix(file.Name(), expectedPrefix)

				// Check that the trim prefix removed the expectedPrefix && there are no other underscores in the testName
				if testName != file.Name() && !strings.Contains(testName, "_") {
					c.Run(testName, func(c *qt.C) {
						language, ok := Detect(file.Name())
						if strings.Contains(file.Name(), "openapi") {
							language, ok = LangOpenAPI, true
						}
						c.Assert(ok, qt.IsTrue, qt.Commentf("Unable to detect language type for %s", file.Name()))

						services := clientgentypes.AllServices(res.Meta)
						generatedClient, err := Client(language, "app", res.Meta, services)
						c.Assert(err, qt.IsNil)

						golden.TestAgainst(c, "goapp/"+file.Name(), string(generatedClient))
					})
				}
			}
		})
	}
}

func TestClientCodeGenerationFromTSApp(t *testing.T) {
	t.Helper()
	c := qt.New(t)

	tests, err := filepath.Glob("./testdata/tsapp/input*.ts")
	if err != nil {
		t.Fatal(err)
	}
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	bld := tsbuilder.New()

	for _, path := range tests {
		path := path
		c.Run("expected"+strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".ts"), "input"), func(c *qt.C) {
			ar, err := txtar.ParseFile(path)
			c.Assert(err, qt.IsNil)

			base := t.TempDir()
			err = txtar.Write(ar, base)
			c.Assert(err, qt.IsNil)

			res, err := bld.Parse(ctx, builder.ParseParams{
				Build:       builder.DefaultBuildInfo(),
				App:         apps.NewInstance(base, "app", ""),
				Experiments: nil,
				WorkingDir:  ".",
				ParseTests:  false,
			})
			c.Assert(err, qt.IsNil)

			files, err := os.ReadDir("./testdata/tsapp")
			c.Assert(err, qt.IsNil)

			expectedPrefix := "expected" + strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".ts"), "input") + "_"

			for _, file := range files {
				testName := strings.TrimPrefix(file.Name(), expectedPrefix)

				// Check that the trim prefix removed the expectedPrefix && there are no other underscores in the testName
				if testName != file.Name() && !strings.Contains(testName, "_") {
					c.Run(testName, func(c *qt.C) {
						language, ok := Detect(file.Name())
						if strings.Contains(file.Name(), "openapi") {
							language, ok = LangOpenAPI, true
						}
						c.Assert(ok, qt.IsTrue, qt.Commentf("Unable to detect language type for %s", file.Name()))

						services := clientgentypes.AllServices(res.Meta)
						generatedClient, err := Client(language, "app", res.Meta, services)
						c.Assert(err, qt.IsNil)

						golden.TestAgainst(c, "tsapp/"+file.Name(), string(generatedClient))
					})
				}
			}
		})
	}
}
