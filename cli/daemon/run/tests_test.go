package run

import (
	"context"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.uber.org/goleak"
)

// TestTestEnvirons tests that user-provided environment variables are propagated to 'go test'.
func TestTestEnvirons(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	c := qt.New(t)

	build := testBuild(c, "./testdata/echo")
	wantEnv := []string{"FOO=bar", "BAR=baz"}
	mgr := &Manager{}

	err := mgr.Test(context.Background(), TestParams{
		Parse:   build.Parse,
		Environ: wantEnv,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		AppRoot: "./testdata/echo",
		Args:    []string{"-count=1", "./..."},
	})
	c.Assert(err, qt.IsNil)
}
