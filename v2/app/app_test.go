package app

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/paths"
)

func TestDesc_ServiceForPath(t *testing.T) {
	c := qt.New(t)
	desc := newTestDesc("/app/a", "/app/b/c", "/app/bc")

	tests := []struct {
		path string
		want string // root of the service found, or "" for none
	}{
		{path: "/app/a", want: "/app/a"},
		{path: "/app/a/x/y", want: "/app/a"},
		{path: "/app/b/c/d", want: "/app/b/c"},
		{path: "/app/bc/x", want: "/app/bc"},
		{path: "/app/a/../b/c", want: "/app/b/c"},
		{path: "/app/b", want: ""},
		{path: "/app", want: ""},
		{path: "/other/a", want: ""},
	}
	for _, tt := range tests {
		c.Run(tt.path, func(c *qt.C) {
			svc, ok := desc.ServiceForPath(paths.FS(tt.path))
			if tt.want == "" {
				c.Assert(ok, qt.IsFalse)
				return
			}
			c.Assert(ok, qt.IsTrue)
			c.Assert(string(svc.FSRoot), qt.Equals, tt.want)
		})
	}
}

func BenchmarkDesc_ServiceForPath(b *testing.B) {
	roots := make([]string, 300)
	for i := range roots {
		roots[i] = fmt.Sprintf("/app/svc%03d", i)
	}
	desc := newTestDesc(roots...)
	path := paths.FS("/app/svc299/internal/store")

	b.ReportAllocs()
	for b.Loop() {
		desc.ServiceForPath(path)
	}
}

func newTestDesc(roots ...string) *Desc {
	services := make([]*Service, len(roots))
	for i, root := range roots {
		services[i] = &Service{Name: root, FSRoot: paths.FS(root)}
	}
	return &Desc{Services: services, serviceByRoot: servicesByRoot(services)}
}
