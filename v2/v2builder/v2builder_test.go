package v2builder

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"

	"encr.dev/pkg/cueutil"
	"encr.dev/v2/app"
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
