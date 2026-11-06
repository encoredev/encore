package paths

import (
	"path/filepath"
	"testing"
)

func TestFSHasPrefix(t *testing.T) {
	ps := []string{"/", "/a", "/a/", "/a/b", "/ab", "/a/bc", "/a/b/c", "/b", "/a/b/../c", "/a//b", "/A/b", "a", "a/b", "."}

	t.Run("matches filepath.Rel", func(t *testing.T) {
		for _, p := range ps {
			for _, dir := range ps {
				rel, err := filepath.Rel(dir, p)
				want := err == nil && filepath.IsLocal(rel)
				if got := FS(p).HasPrefix(FS(dir)); got != want {
					t.Errorf("FS(%q).HasPrefix(%q) = %v, want %v", p, dir, got, want)
				}
			}
		}
	})

	t.Run("clean absolute paths do not allocate", func(t *testing.T) {
		if filepath.Separator != '/' {
			t.Skip("Windows paths go through filepath.Rel")
		}
		// Most calls are a package checked against a service it is not in,
		// where filepath.Rel builds a "../.." path.
		p := FS("/app/svc/internal/store")
		for _, tc := range []struct {
			dir  FS
			want bool
		}{
			{dir: "/app/svc", want: true},
			{dir: "/app/other/svc", want: false},
		} {
			allocs := testing.AllocsPerRun(100, func() {
				if got := p.HasPrefix(tc.dir); got != tc.want {
					t.Fatalf("HasPrefix(%q) = %v, want %v", tc.dir, got, tc.want)
				}
			})
			if allocs != 0 {
				t.Errorf("HasPrefix(%q) allocated %v times per call, want 0", tc.dir, allocs)
			}
		}
	})
}
