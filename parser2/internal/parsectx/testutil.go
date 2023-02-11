package parsectx

import (
	"context"
	"fmt"
	"go/build"
	"go/token"

	qt "github.com/frankban/quicktest"
	"github.com/rs/zerolog"

	"encr.dev/parser2/internal/perr"
)

// NewForTest constructs a new Context for testing.
// It defaults the build info to the host system.
func NewForTest(c *qt.C, parseTests bool) *Context {
	d := &build.Default
	info := BuildInfo{
		GOARCH:     d.GOARCH,
		GOOS:       d.GOOS,
		GOROOT:     d.GOROOT,
		BuildTags:  nil,
		CgoEnabled: true,
	}

	fset := token.NewFileSet()
	ctx, cancel := context.WithCancelCause(context.Background())
	c.Cleanup(func() {
		cancel(fmt.Errorf("test %s aborted", c.Name()))
	})

	return &Context{
		Log:        zerolog.New(zerolog.NewConsoleWriter(zerolog.ConsoleTestWriter(c))),
		Ctx:        ctx,
		Build:      info,
		FS:         fset,
		ParseTests: parseTests, // might as well
		Errs:       perr.NewList(ctx, fset),
		c:          c,
	}
}

// FailTestOnErrors is function that fails the test if an errors
// are encountered when running fn.
func FailTestOnErrors(ctx *Context, fn func()) {
	if ctx.c == nil {
		panic("parsectx.Context created outside of NewForTest")
	}
	defer FailTestOnBailout(ctx.c)
	ctx.Errs.BailoutOnErrors(fn)
}

// FailTestOnBailout is a defer function that fails the test if a bailout
// was triggered. It must be called as "defer FailTestOnBailout(ctx)".
func FailTestOnBailout(c *qt.C) {
	if l, caught := perr.CatchBailout(recover()); caught {
		c.Fatalf("bailout: %s", l.FormatErrors())
	}
}
