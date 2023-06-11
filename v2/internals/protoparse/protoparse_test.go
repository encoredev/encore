package protoparse

import (
	"context"
	"go/token"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/perr"
)

func TestParse(t *testing.T) {
	c := qt.New(t)
	wd, err := os.Getwd()
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	fset := token.NewFileSet()
	errs := perr.NewList(ctx, fset)

	pp := NewParser(
		errs,
		[]paths.FS{paths.RootedFSPath(wd, "testdata")},
	)

	errs.BailoutOnErrors(func() {
		pp.parseFile(ctx, nil, "helloworld.proto")
	})
}
