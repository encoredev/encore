package run

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"encr.dev/pkg/watcher"
)

func TestIgnoreEvent(t *testing.T) {
	c := qt.New(t)
	testCases := map[string]bool{
		"/app/svc/api.go":           false,
		"/app/go.mod":               false,
		"/app/go.sum":               false,
		"/app/go.work":              false,
		"/app/svc/migrations/1.sql": false,
		"/app/go.work.sum":          true,
		"/app/svc/encore.gen.go":    true,
		"/app/README.md":            true,
	}
	for path, want := range testCases {
		c.Run(path, func(c *qt.C) {
			c.Assert(ignoreEvent(watcher.Event{Path: path}), qt.Equals, want)
		})
	}
}
