package parser2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"

	"encr.dev/parser2/internal/paths"
)

func TestWalkDirs(t *testing.T) {
	tests := []struct {
		// Tree represents a directory tree. See createTree.
		Tree string
		// Pkgs are the packages yielded by the walk,
		// separated by space.
		Pkgs string
	}{
		{"a", ""},
		{"foo/", ""},
		{"a.go", "."},
		{"a.go foo/b.go foo/bar/c foo/bar/baz/d.go", ". foo foo/bar/baz"},
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
		basePkgPath := paths.MustPkgPath("x")
		var got []paths.Pkg
		err := walkGoPackages(paths.RootedFSPath(root, "."), basePkgPath, func(p paths.Pkg) {
			got = append(got, p)
		})
		c.Assert(err, qt.IsNil)

		// Compare the packages
		wantPkgs := strings.Fields(test.Pkgs)
		want := make([]paths.Pkg, len(wantPkgs))
		for i, p := range wantPkgs {
			want[i] = basePkgPath.JoinSlash(p)
		}
		c.Assert(got, qt.CmpEquals(cmpopts.EquateEmpty()), want, qt.Commentf("tree: %s", test.Tree))
	}
}
