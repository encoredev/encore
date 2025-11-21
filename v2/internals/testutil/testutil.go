package testutil

import (
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp"
	"github.com/rogpeppe/go-internal/testscript"
	"github.com/rogpeppe/go-internal/txtar"
	"github.com/rs/zerolog"

	"encr.dev/internal/env"
	"encr.dev/pkg/errinsrc"
	"encr.dev/pkg/option"
	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/parsectx"
	"encr.dev/v2/internals/perr"
	"encr.dev/v2/internals/pkginfo"
)

type Context struct {
	*parsectx.Context
	TestC *qt.C
}

// NewContext constructs a new Context for testing.
// It defaults the build info to the host system.
func NewContext(c *qt.C, parseTests bool, archive *txtar.Archive) *Context {
	mainModuleDir := WriteTxtar(c, archive)

	return newContextForFSPath(c, mainModuleDir, parseTests)
}

// NewContextForTestScript constructs a new Context for testing when using testscript
//
// Your testscript test should call this [TestScriptSetupFunc] in the TestScript.Setup function.
func NewContextForTestScript(ts *testscript.TestScript, parseTests bool) *Context {
	c := GetTestC(ts)
	workdir := ts.Value("wd").(string)

	// Write the go.mod file for the testscript as we don't expect each test to do this
	additional := ParseTxtar(`-- go.mod --
module test

go 1.20

require encore.dev v1.13.4

-- fakesvcfortest/test.go --
// this service only exists to suppress the "no services found error"
package fakesvcfortest

import "context"

//encore:api public
func TestFunc(ctx context.Context) error { return nil }`)
	ts.Check(txtar.Write(additional, workdir))

	return newContextForFSPath(c, workdir, parseTests)
}

// TestScriptSetupFunc is a testscript setup function which sets up the testscript environment for
// testing with testutil.
func TestScriptSetupFunc(env *testscript.Env) error {
	env.Values["TESTUTIL_SCRIPT_SETUP"] = true
	env.Values["wd"] = env.WorkDir

	tb, ok := env.T().(testing.TB)
	if !ok {
		env.T().Fatal("testscript's T did not implement testing.TB as expected")
	}
	env.Values["c"] = qt.New(tb)

	return nil
}

// GetTestC returns the *qt.C for the current testscript test.
//
// This should only be called from within a testscript test which has had [TestScriptSetupFunc] called
// during the TestScript.Setup function.
func GetTestC(ts *testscript.TestScript) *qt.C {
	if value := ts.Value("TESTUTIL_SCRIPT_SETUP"); value != nil {
		if b, ok := value.(bool); !ok || !b {
			ts.Fatalf("testutil.TestScriptSetupFunc was not called in the TestScript.Setup function")
		}
	}

	return ts.Value("c").(*qt.C)
}

func newContextForFSPath(c *qt.C, mainModuleDir string, parseTests bool) *Context {
	errinsrc.ColoursInErrors(false) // disable colours in errors for tests
	errinsrc.IncludeStackByDefault = true

	ctx, cancel := context.WithCancelCause(context.Background())
	c.Cleanup(func() {
		cancel(fmt.Errorf("test %s aborted", c.Name()))
	})

	d := &build.Default
	info := parsectx.BuildInfo{
		Experiments:   nil,
		GOARCH:        d.GOARCH,
		GOOS:          d.GOOS,
		GOROOT:        paths.RootedFSPath(env.EncoreGoRoot(), "."),
		BuildTags:     nil,
		CgoEnabled:    true,
		EncoreRuntime: paths.RootedFSPath(filepath.Join(RuntimeDir, "go"), "."),
	}

	fset := token.NewFileSet()
	parseCtx := &parsectx.Context{
		Log:           zerolog.New(zerolog.NewConsoleWriter(zerolog.ConsoleTestWriter(c))).Level(zerolog.InfoLevel),
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

	for i := range n {
		err := l.At(i)
		re := regexp.MustCompile(matches[i])
		errMsg := err.Error()
		c.TestC.Check(re.MatchString(errMsg), qt.IsTrue, qt.Commentf("err %v does not match regexp %q",
			errMsg, matches[i]))
	}
}

// GoModTidy runs "go mod tidy" on the main module.
func (c *Context) GoModTidy() {
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(c.goBin(), "mod", "tidy")
	cmd.Dir = c.MainModuleDir.ToIO()
	c.TestC.Log("running 'go mod tidy'")
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		c.TestC.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	c.TestC.Log("'go mod tidy' completed successfully")
}

// GoModDownload runs "go mod download all" on the main module.
func (c *Context) GoModDownload() {
	// The "all" arg is needed to force 'go mod download' to update
	// the go.sum file. See https://go-review.googlesource.com/c/go/+/318629.

	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(c.goBin(), "mod", "download", "all")
	cmd.Dir = c.MainModuleDir.ToIO()
	c.TestC.Log("running 'go mod download'")
	// nosemgrep
	if out, err := cmd.CombinedOutput(); err != nil {
		c.TestC.Fatalf("go mod download: %v\n%s", err, out)
	}
	c.TestC.Log("'go mod download' completed successfully")
}

func (c *Context) goBin() string {
	s := c.Build.GOROOT.Join("bin", "go").ToIO()
	if runtime.GOOS == "windows" {
		s += ".exe"
	}
	return s
}

func ParseTxtar(s string) *txtar.Archive {
	return txtar.Parse([]byte(s))
}

// WriteTxtar writes the given txtar archive to a temporary directory
// and returns the directory path.
func WriteTxtar(c *qt.C, a *txtar.Archive) (dir string) {
	c.Helper()
	dir = c.TempDir()

	// NOTE(andre): There appears to be a bug in go's handling of overlays
	// when the source or destination is a symlink.
	// I haven't dug into the root cause exactly, but it causes weird issues
	// with tests since macOS's /var/tmp is a symlink to /private/var/tmp.
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}

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

// A PackageList is a list of packages that knows how to
// collect itself, by passing (*PackageList).Collector() to
// the scan.ProcessModule function. It's primarily a helper function
// for testing purposes.
type PackageList []*pkginfo.Package

func (l *PackageList) Collector() func(pkg *pkginfo.Package) {
	var mu sync.Mutex
	return func(pkg *pkginfo.Package) {
		mu.Lock()
		*l = append(*l, pkg)
		mu.Unlock()
	}
}

// ResourceDeepEquals is a quicktest comparator for resource.Resource and resource.Bind
// that forces the comparison to include unexported fields
var ResourceDeepEquals = qt.CmpEquals(
	cmp.AllowUnexported(option.Option[*pkginfo.File]{}),
)
