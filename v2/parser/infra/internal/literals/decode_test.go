package literals

import (
	"go/ast"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

func TestDecode(t *testing.T) {
	c := qt.New(t)
	tc := testutil.NewContext(c, false, testutil.ParseTxtar(`
-- go.mod --
module example.com
require encore.dev v1.52.0
-- foo.go --
package foo

import ("time"; "encore.dev/pubsub")

var x = pubsub.SubscriptionConfig{
	AckDeadline: 45 * time.Second,
	MessageRetention: 5 * time.Hour * 24 + -10 * time.Hour,
	RetryPolicy: &pubsub.RetryPolicy{
		MaxRetries: 3,
		MinBackoff: 8 * time.Second,
		MaxBackoff: 32 * time.Minute,
	},
}
`))
	tc.FailTestOnErrors()
	tc.GoModTidy()

	loader := pkginfo.New(tc.Context)
	pkg := loader.MustLoadPkg(0, "example.com")

	cfgLit, ok := ParseStruct(tc.Errs, pkg.Files[0], "pubsub.SubscriptionConfig",
		pkg.Names().PkgDecls["x"].Spec.(*ast.ValueSpec).Values[0])
	c.Assert(ok, qt.IsTrue)

	type decodedConfig struct {
		// Optional configuration
		AckDeadline      time.Duration `literal:",optional"`
		MessageRetention time.Duration `literal:",optional"`
		RetryPolicy      struct {
			MinRetryBackoff time.Duration `literal:"MinBackoff,optional"`
			MaxRetryBackoff time.Duration `literal:"MaxBackoff,optional"`
			MaxRetries      int           `literal:"MaxRetries,optional"`
		} `literal:",optional"`
	}

	cfg := Decode[decodedConfig](tc.Errs, cfgLit, nil)

	c.Assert(cfg, qt.DeepEquals, decodedConfig{
		AckDeadline:      45 * time.Second,
		MessageRetention: 5*time.Hour*24 + -10*time.Hour,
		RetryPolicy: struct {
			MinRetryBackoff time.Duration `literal:"MinBackoff,optional"`
			MaxRetryBackoff time.Duration `literal:"MaxBackoff,optional"`
			MaxRetries      int           `literal:"MaxRetries,optional"`
		}{
			MaxRetries:      3,
			MinRetryBackoff: 8 * time.Second,
			MaxRetryBackoff: 32 * time.Minute,
		},
	})

}
