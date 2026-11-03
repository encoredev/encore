package pkginfo_test

import (
	"encoding/json"
	"go/token"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"

	"encr.dev/pkg/paths"
	"encr.dev/v2/internals/pkginfo"
	"encr.dev/v2/internals/testutil"
)

// TestLoader_NestedModuleNotDownloaded covers a module cache holding a
// module's source but not that of a module nested in it, as a module cache
// restored from an earlier build can. `go list -m all` reports the nested
// module without a Dir; its packages must still resolve to it (and get
// downloaded), not to the parent module that is on disk.
func TestLoader_NestedModuleNotDownloaded(t *testing.T) {
	proxy := writeModuleProxy(t, map[string]map[string]string{
		"example.com/a": {
			"go.mod": "module example.com/a\n",
			"a.go":   "package a\n",
		},
		"example.com/a/b": {
			"go.mod":       "module example.com/a/b\n",
			"b.go":         "package b\n",
			"blah/blah.go": "package blah\n",
		},
		// Sorts between example.com/a/b and example.com/a/b/blah.
		"example.com/a/b.x": {
			"go.mod": "module example.com/a/b.x\n",
			"bx.go":  "package bx\n",
		},
	})

	tests := []struct {
		name    string
		cached  []string // modules downloaded before parsing
		pkg     paths.Pkg
		wantDir string // relative to the module cache
	}{
		{
			name:    "nested_module",
			cached:  []string{"example.com/a"},
			pkg:     "example.com/a/b",
			wantDir: "example.com/a/b@v1.0.0",
		},
		{
			name:    "package_in_nested_module",
			cached:  []string{"example.com/a"},
			pkg:     "example.com/a/b/blah",
			wantDir: "example.com/a/b@v1.0.0/blah",
		},
		{
			name:    "sibling_module_sorted_in_between",
			cached:  []string{"example.com/a", "example.com/a/b.x"},
			pkg:     "example.com/a/b/blah",
			wantDir: "example.com/a/b@v1.0.0/blah",
		},
		{
			name:    "parent_module",
			cached:  []string{"example.com/a"},
			pkg:     "example.com/a",
			wantDir: "example.com/a@v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			modCache := t.TempDir()
			t.Setenv("GOPROXY", proxy)
			t.Setenv("GOSUMDB", "off")
			t.Setenv("GOWORK", "off")
			t.Setenv("GOMODCACHE", modCache)
			// -mod=mod fills in go.sum; -modcacherw lets t.TempDir clean up.
			t.Setenv("GOFLAGS", "-mod=mod -modcacherw")

			tc := testutil.NewContext(c, false, testutil.ParseTxtar(`
-- go.mod --
module app

go 1.22

require (
	example.com/a v1.0.0
	example.com/a/b v1.0.0
	example.com/a/b.x v1.0.0
)
`))
			tc.FailTestOnErrors()
			defer tc.FailTestOnBailout()

			goModDownload(c, tc, tt.cached...)

			l := pkginfo.New(tc.Context)
			pkg, ok := l.LoadPkg(token.NoPos, tt.pkg)
			c.Assert(ok, qt.IsTrue)
			c.Check(pkg.FSPath.ToIO(), qt.Equals, filepath.Join(modCache, filepath.FromSlash(tt.wantDir)))
		})
	}
}

// writeModuleProxy writes a file:// GOPROXY serving each module, with the
// given files, at v1.0.0, and returns its URL.
func writeModuleProxy(t *testing.T, modules map[string]map[string]string) string {
	t.Helper()
	c := qt.New(t)
	root := t.TempDir()
	for modPath, files := range modules {
		src := t.TempDir()
		for name, data := range files {
			p := filepath.Join(src, filepath.FromSlash(name))
			c.Assert(os.MkdirAll(filepath.Dir(p), 0o755), qt.IsNil)
			c.Assert(os.WriteFile(p, []byte(data), 0o644), qt.IsNil)
		}

		mv := module.Version{Path: modPath, Version: "v1.0.0"}
		dir := filepath.Join(root, filepath.FromSlash(modPath), "@v")
		c.Assert(os.MkdirAll(dir, 0o755), qt.IsNil)

		info, err := json.Marshal(map[string]string{"Version": mv.Version, "Time": "2024-01-01T00:00:00Z"})
		c.Assert(err, qt.IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, "list"), []byte(mv.Version+"\n"), 0o644), qt.IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, mv.Version+".info"), info, 0o644), qt.IsNil)
		c.Assert(os.WriteFile(filepath.Join(dir, mv.Version+".mod"), []byte(files["go.mod"]), 0o644), qt.IsNil)

		zf, err := os.Create(filepath.Join(dir, mv.Version+".zip"))
		c.Assert(err, qt.IsNil)
		c.Assert(modzip.CreateFromDir(zf, mv, src), qt.IsNil)
		c.Assert(zf.Close(), qt.IsNil)
	}

	p := filepath.ToSlash(root)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // a Windows drive path
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// goModDownload downloads the given modules into the module cache.
func goModDownload(c *qt.C, tc *testutil.Context, mods ...string) {
	if len(mods) == 0 {
		return
	}
	goBin := tc.Build.GOROOT.Join("bin", "go").ToIO()
	if runtime.GOOS == "windows" {
		goBin += ".exe"
	}
	// nosemgrep go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(goBin, append([]string{"mod", "download"}, mods...)...)
	cmd.Dir = tc.MainModuleDir.ToIO()
	// nosemgrep
	out, err := cmd.CombinedOutput()
	c.Assert(err, qt.IsNil, qt.Commentf("go mod download: %s", out))
}
