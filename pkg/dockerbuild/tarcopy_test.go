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
