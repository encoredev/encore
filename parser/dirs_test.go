package parser

import (
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/rogpeppe/go-internal/txtar"
)

func TestWalkDirs(t *testing.T) {
	type call struct {
		Dir   string
		Path  string
		Files []string
	}
	tests := []struct {
		// Tree represents a directory tree. See createTree.
		Tree string
		// Calls are the expected calls to walkFn, in order.
		Calls []call
	}{
		{"a", []call{{"", ".", []string{"a"}}}},
		{"a/", []call{
			{"", ".", []string{}},
			{"a", "a", []string{}},
		}},
		{"a/b", []call{
			{"", ".", []string{}},
			{"a", "a", []string{"b"}},
		}},
		{"a/b a/c a/d/e", []call{
			{"", ".", []string{}},
			{"a", "a", []string{"b", "c"}},
			{"a/d", "a/d", []string{"e"}},
		}},
	}

	// createTree creates the directory tree represented by tree.
	// Nodes are space-separated; dirs end with a trailing slash.
	//
	// "a b/c d/" represents a root with one file "a",
	// the directory "b" (containing file "c") and an empty directory "d".
	// It reports the directory root.
	createTree := func(tree string) (root string) {
		root = t.TempDir()
		for _, node := range strings.Fields(tree) {
			// Create the dir if we have a slash
			if idx := strings.LastIndexByte(node, '/'); idx > 0 {
				p := filepath.Join(root, filepath.FromSlash(node[:idx]))
				if err := os.MkdirAll(p, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if !strings.HasSuffix(node, "/") {
				f, err := os.Create(filepath.Join(root, filepath.FromSlash(node)))
				if err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
		}
		return root
	}

	c := qt.New(t)
	for _, test := range tests {
		root := createTree(test.Tree)
		var calls []call
		walkDirs(root, func(dir, relPath string, files []os.FileInfo) error {
			names := make([]string, len(files))
			for i, f := range files {
				names[i] = f.Name()
			}
			if dir == root {
				dir = ""
			} else {
				dir = dir[len(root)+1:]
			}
			calls = append(calls, call{dir, relPath, names})
			return nil
		})

		expect := make([]call, len(test.Calls))
		for i, c := range test.Calls {
			expect[i] = call{c.Dir, c.Path, c.Files}
		}
		c.Assert(calls, qt.DeepEquals, expect, qt.Commentf("tree: %s", test.Tree))
	}
}

func TestParseDir(t *testing.T) {
	tests := []struct {
		Archive  string
		PkgNames []string
		NFiles   int
		Err      string
	}{
		{
			Archive: `
-- foo.go --
package foo
-- bar.go --
package bar
`,
			PkgNames: []string{"foo", "bar"},
			NFiles:   2,
		},
		{
			Archive: `
-- foo.go --
package fo/;
`,
			Err: ".*foo.go:.*expected ';', found '/'",
		},
	}

	c := qt.New(t)
	for i, test := range tests {
		a := txtar.Parse([]byte(test.Archive))
		base := t.TempDir()
		err := txtar.Write(a, base)
		c.Assert(err, qt.IsNil, qt.Commentf("test #%d", i))

		fs := token.NewFileSet()
		pkgs, files, err := parseDir(fs, base, ".", nil, goparser.ParseComments)
		if test.Err != "" {
			c.Assert(err, qt.ErrorMatches, test.Err)
			continue
		}
		c.Assert(err, qt.IsNil)
		var pkgNames []string
		for name := range pkgs {
			pkgNames = append(pkgNames, name)
		}
		c.Assert(pkgNames, qt.ContentEquals, test.PkgNames)
		c.Assert(files, qt.HasLen, test.NFiles)
	}
}
