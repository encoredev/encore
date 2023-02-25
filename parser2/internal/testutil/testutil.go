package testutil

import (
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"os/exec"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encr.dev/parser2/internal/parsectx"
	"encr.dev/parser2/internal/paths"
	"encr.dev/parser2/internal/perr"
)

type Context struct {
	*parsectx.Context
	TestC *qt.C
}

// NewContext constructs a new Context for testing.
// It defaults the build info to the host system.
func NewContext(c *qt.C, parseTests bool, archive *txtar.Archive) *Context {
	mainModuleDir := WriteTxtar(c, archive)

	d := &build.Default
	info := parsectx.BuildInfo{
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

	parseCtx := &parsectx.Context{
		Log:           zerolog.New(zerolog.NewConsoleWriter(zerolog.ConsoleTestWriter(c))),
		MainModuleDir: paths.RootedFSPath(mainModuleDir, mainModuleDir),
		Ctx:           ctx,
		Build:         info,
		FS:            fset,
		ParseTests:    parseTests, // might as well
		Errs:          perr.NewList(ctx, fset),
	}

	return &Context{Context: parseCtx, TestC: c}
}

// FailTestOnErrors is function that fails the test if an errors
// are encountered when running fn.
func (c *Context) FailTestOnErrors() {
	if c.TestC == nil {
		panic("parsectx.Context created outside of NewContext")
	}

	n := c.Errs.Len()
	c.TestC.Cleanup(func() {
		if c.Errs.Len() > n {
			c.TestC.Fatalf("parse errors: %s", c.Errs.FormatErrors())
		}
	})
}

// FailTestOnBailout is a defer function that fails the test if a bailout
// was triggered. It must be called as "defer c.FailTestOnBailout()".
func (c *Context) FailTestOnBailout() {
	c.TestC.Helper()
	if l, caught := perr.CatchBailout(recover()); caught {
		c.TestC.Fatalf("bailout: %s", l.FormatErrors())
	}
}

// DeferExpectError is a defer function that expects errors to be present.
// Each argument is matched with the corresponding error.
// If the number of errors differs from the number of matches the test fails.
func (c *Context) DeferExpectError(matches ...string) {
	// Ignore any bailout; we'll check the list directly.
	perr.CatchBailout(recover())

	l := c.Errs
	n := l.Len()
	if len(matches) != n {
		c.TestC.Fatalf("expected %d errors, got %d: %s", len(matches), n, l.FormatErrors())
	}

	for i := 0; i < n; i++ {
		err := l.At(i)
		c.TestC.Check(err, qt.ErrorMatches, matches[i])
	}
}

// GoModTidy runs "go mod tidy" on the main module.
func (c *Context) GoModTidy() {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = c.MainModuleDir.ToIO()
	c.TestC.Log("running 'go mod tidy'")
	if out, err := cmd.CombinedOutput(); err != nil {
		c.TestC.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	c.TestC.Log("'go mod tidy' completed successfully")
}

// GoModDownload runs "go mod download all" on the main module.
func (c *Context) GoModDownload() {
	// The "all" arg is needed to force 'go mod download' to update
	// the go.sum file. See https://go-review.googlesource.com/c/go/+/318629.
	cmd := exec.Command("go", "mod", "download", "all")
	cmd.Dir = c.MainModuleDir.ToIO()
	c.TestC.Log("running 'go mod download'")
	if out, err := cmd.CombinedOutput(); err != nil {
		c.TestC.Fatalf("go mod download: %v\n%s", err, out)
	}
	c.TestC.Log("'go mod download' completed successfully")
}

func ParseTxtar(s string) *txtar.Archive {
	return txtar.Parse([]byte(s))
}

// WriteTxtar writes the given txtar archive to a temporary directory
// and returns the directory path.
func WriteTxtar(c *qt.C, a *txtar.Archive) (dir string) {
	c.Helper()
	dir = c.TempDir()
	err := txtar.Write(a, dir)
	c.Assert(err, qt.IsNil)
	return dir
}

// FindNodes finds all nodes of the type T in the given AST.
func FindNodes[T ast.Node](root ast.Node) []T {
	var results []T
	ast.Inspect(root, func(n ast.Node) bool {
		if t, ok := n.(T); ok {
			results = append(results, t)
		}
		return true
	})
	return results
}
