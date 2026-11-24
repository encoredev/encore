package v2builder

import (
	"context"
	"go/token"
	"os"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/cueutil"
	"encr.dev/pkg/paths"
	"encr.dev/v2/app"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/testutil"
	"encr.dev/v2/parser"
)

const configApp = `
-- go.mod --
module encore.app
require encore.dev v1.52.0
-- svc/svc.go --
package svc

import (
	"context"

	"encore.dev/config"
)

type Config struct {
	CallbackURL string
}

var _ = config.Load[*Config]()

//encore:api public
func MyAPI(ctx context.Context) error {
	return nil
}
-- svc/svc.cue --
CallbackURL: "\(#Meta.APIBaseURL)/svc.MyAPI"
`

// TestComputeConfigs_GeneratedCUE verifies that the CUE definitions the config
// is validated against come from the parse result, so builds don't depend on
// the encore.gen.cue that GenUserFacing writes to disk for editors: it may be
// absent, as in a fresh checkout, or stale.
func TestComputeConfigs_GeneratedCUE(t *testing.T) {
	tests := []struct {
		name   string
		onDisk string // contents of svc/encore.gen.cue, if any
	}{
		{
			name: "missing",
		},
		{
			name: "stale",
			// Declares a config schema that the app no longer has, and no
			// #Meta, so loading it would fail on the missing APIBaseURL tag.
			onDisk: "package svc\n\n#Config: {\n\tRemovedField: string\n}\n\n#Config\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)
			a := testutil.ParseTxtar(configApp)
			if test.onDisk != "" {
				a.Files = append(a.Files, txtar.File{
					Name: "svc/encore.gen.cue",
					Data: []byte(test.onDisk),
				})
			}

			tc := testutil.NewContext(c, false, a)
			tc.GoModDownload()
			tc.FailTestOnErrors()
			defer tc.FailTestOnBailout()

			p := parser.NewParser(tc.Context)
			result := p.Parse()
			desc := app.ValidateAndDescribe(tc.Context, result)

			cfg := computeConfigs(tc.Errs, desc, p.MainModule(), &cueutil.Meta{
				APIBaseURL: "http://localhost:4000",
				EnvName:    "local",
				EnvType:    cueutil.EnvType_Development,
				CloudType:  cueutil.CloudType_Local,
			})

			c.Assert(cfg.configs["svc"], qt.JSONEquals, map[string]any{
				"CallbackURL": "http://localhost:4000/svc.MyAPI",
			})
		})
	}
}

// TestWriteOrDeleteFile verifies that a generated file that already holds the
// data is not rewritten, so its modification time does not change.
func TestWriteOrDeleteFile(t *testing.T) {
	c := qt.New(t)
	dir := t.TempDir()
	dst := paths.RootedFSPath(dir, ".").Join("encore.gen.go")
	errs := perr.NewList(context.Background(), token.NewFileSet())
	b := &BuilderImpl{}

	// modTimeAfterWrite backdates the file, writes data and returns the file's
	// modification time, so a rewrite shows up whatever the clock's resolution.
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	modTimeAfterWrite := func(data string) time.Time {
		c.Assert(os.Chtimes(dst.ToIO(), old, old), qt.IsNil)
		b.writeOrDeleteFile(errs, []byte(data), dst)
		c.Assert(errs.Len(), qt.Equals, 0)
		fi, err := os.Stat(dst.ToIO())
		c.Assert(err, qt.IsNil)
		return fi.ModTime()
	}

	// A missing file is written.
	b.writeOrDeleteFile(errs, []byte("package svc\n"), dst)
	c.Assert(errs.Len(), qt.Equals, 0)

	// The same contents leave the file alone.
	c.Assert(modTimeAfterWrite("package svc\n").Equal(old), qt.IsTrue)

	// New contents are written.
	c.Assert(modTimeAfterWrite("package svc\n\n// changed\n").Equal(old), qt.IsFalse)
	got, err := os.ReadFile(dst.ToIO())
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, "package svc\n\n// changed\n")

	// No contents remove the file.
	b.writeOrDeleteFile(errs, nil, dst)
	c.Assert(errs.Len(), qt.Equals, 0)
	_, err = os.Stat(dst.ToIO())
	c.Assert(os.IsNotExist(err), qt.IsTrue)
}
