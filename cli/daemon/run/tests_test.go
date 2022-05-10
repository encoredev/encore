package run

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.uber.org/goleak"
	"golang.org/x/mod/modfile"

	"encr.dev/parser"
)

// TestTests tests that tests can successfully be run.
func TestTests(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	c := qt.New(t)
	ctx := context.Background()

	appRoot := "./testdata/echo"
	parse := testParse(c, appRoot, true)

	mgr := &Manager{}
	err := mgr.Test(ctx, TestParams{
		AppRoot:    "./testdata/echo",
		WorkingDir: ".",
		Parse:      parse,
		Args:       []string{"./..."},
		Stdout:     os.Stdout,
		Stderr:     os.Stdout,
	})
	c.Assert(err, qt.IsNil)
}

func testParse(c *qt.C, appRoot string, parseTests bool) *parser.Result {
	modPath := filepath.Join(appRoot, "go.mod")
	modData, err := ioutil.ReadFile(modPath)
	c.Assert(err, qt.IsNil)
	mod, err := modfile.Parse(modPath, modData, nil)
	c.Assert(err, qt.IsNil)

	cfg := &parser.Config{
		AppRoot:     appRoot,
		AppRevision: "",
		ModulePath:  mod.Module.Mod.Path,
		WorkingDir:  ".",
		ParseTests:  parseTests,
	}
	res, err := parser.Parse(cfg)
	c.Assert(err, qt.IsNil)
	return res
}
