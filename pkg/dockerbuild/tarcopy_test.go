package dockerbuild

import (
	"io/fs"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestMkdirAll_ParentBeforeChild(t *testing.T) {
	tests := []struct {
		name     string
		path     ImagePath
		wantDirs []string // expected tar header names in order
	}{
		{
			name:     "deep_path",
			path:     "/a/b/c/d",
			wantDirs: []string{"a/", "a/b/", "a/b/c/", "a/b/c/d/"},
		},
		{
			name:     "single_component",
			path:     "/app",
			wantDirs: []string{"app/"},
		},
		{
			name:     "relative_path",
			path:     "encore/runtimes/js",
			wantDirs: []string{"encore/", "encore/runtimes/", "encore/runtimes/js/"},
		},
		{
			name:     "varying_segment_lengths",
			path:     "/a/bbbb/cc",
			wantDirs: []string{"a/", "a/bbbb/", "a/bbbb/cc/"},
		},
		{
			name:     "root",
			path:     "/",
			wantDirs: nil,
		},
		{
			name:     "dot",
			path:     ".",
			wantDirs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			tc := newTarCopier()

			err := tc.MkdirAll(tt.path, fs.FileMode(0755))
			c.Assert(err, qt.IsNil)

			var got []string
			for _, e := range tc.entries {
				got = append(got, e.header.Name)
			}
			c.Assert(got, qt.DeepEquals, tt.wantDirs)
		})
	}
}

func TestMkdirAll_Deduplication(t *testing.T) {
	c := qt.New(t)
	tc := newTarCopier()

	// First call creates all ancestors.
	err := tc.MkdirAll("/app/svc/handler", fs.FileMode(0755))
	c.Assert(err, qt.IsNil)
	c.Assert(len(tc.entries), qt.Equals, 3) // app/, app/svc/, app/svc/handler/

	// Second call with a sibling path should only add the new leaf.
	err = tc.MkdirAll("/app/svc/models", fs.FileMode(0755))
	c.Assert(err, qt.IsNil)
	c.Assert(len(tc.entries), qt.Equals, 4) // + app/svc/models/

	// Verify no duplicates and correct ordering across all entries.
	var got []string
	for _, e := range tc.entries {
		got = append(got, e.header.Name)
	}
	c.Assert(got, qt.DeepEquals, []string{
		"app/",
		"app/svc/",
		"app/svc/handler/",
		"app/svc/models/",
	})
}

func TestNodeModulesPath(t *testing.T) {
	tests := []struct {
		path   HostPath
		within bool
		isRoot bool
	}{
		{"node_modules", true, true},
		{"node_modules/foo", true, false},
		{"app/node_modules", true, true},
		{"app/node_modules/pkg", true, false},
		{"app/node_modules/pkg/node_modules", true, false},
		{"app/src/index.ts", false, false},
		{"node_modules_backup/foo", false, false},
		{".", false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.path), func(t *testing.T) {
			c := qt.New(t)
			within, isRoot := nodeModulesPath(tt.path)
			c.Assert(within, qt.Equals, tt.within)
			c.Assert(isRoot, qt.Equals, tt.isRoot)
		})
	}
}

func TestIsVolatileDepMetadata(t *testing.T) {
	tests := []struct {
		path HostPath
		want bool
	}{
		// Volatile bookkeeping files directly inside a node_modules dir.
		{"node_modules/.modules.yaml", true},
		{"node_modules/.pnpm-workspace-state.json", true},
		{"node_modules/.pnpm-workspace-state-v1.json", true},
		{"app/node_modules/.modules.yaml", true},
		// Nested node_modules roots (e.g. pnpm virtual store) are still caught.
		{"node_modules/.pnpm/pkg@1.0.0/node_modules/.modules.yaml", true},
		// Not directly under node_modules -> a package may legitimately ship
		// a coincidentally-named file deep inside it; must not be skipped.
		{"node_modules/pkg/fixtures/.modules.yaml", false},
		{"node_modules/pkg/.modules.yaml", false},
		// At the workspace root (no node_modules parent) -> keep.
		{".modules.yaml", false},
		// Regular dependency files -> keep.
		{"node_modules/foo", false},
		{"node_modules/pkg/index.js", false},
		// Not in the denylist -> keep.
		{"node_modules/package.json", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.path), func(t *testing.T) {
			c := qt.New(t)
			c.Assert(isVolatileDepMetadata(tt.path), qt.Equals, tt.want)
		})
	}
}

func TestMkdirAll_OrderingInvariant(t *testing.T) {
	c := qt.New(t)
	tc := newTarCopier()

	// Create a deep path to verify that every entry's parent appears before it.
	err := tc.MkdirAll("/encore/runtimes/js/node_modules/pkg", fs.FileMode(0755))
	c.Assert(err, qt.IsNil)

	seen := make(map[string]bool)
	for _, e := range tc.entries {
		name := strings.TrimSuffix(e.header.Name, "/")
		// Every entry except the top-level should have its parent already seen.
		if idx := strings.LastIndex(name, "/"); idx > 0 {
			parent := name[:idx] + "/"
			c.Assert(seen[parent], qt.IsTrue, qt.Commentf("parent %q not seen before child %q", parent, e.header.Name))
		}
		seen[e.header.Name] = true
	}
}
