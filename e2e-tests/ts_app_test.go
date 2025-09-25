//go:build e2e

package tests

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestTSEndToEndWithApp(t *testing.T) {
	c := qt.New(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	nodePath, ok := getNodeJSPath().Get()
	if !ok {
		c.Fatal("Could not find nodejs binary, it is needed to run typescript apps")
	}

	appRoot := filepath.Join(wd, "testdata", "tsapp")
	app := RunApp(c, appRoot, nil, []string{"PATH=" + nodePath})
	run := app.Run

	c.Run("run tests", func(c *qt.C) {
		err := RunTests(c.TB, appRoot, os.Stdout, os.Stderr, nil)
		c.Assert(err, qt.IsNil)
	})

	c.Run("typescript hello endpoint", func(c *qt.C) {
		// Test the TypeScript hello endpoint
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/hello/world", nil)
		run.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, 200)
		c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]string{
			"message": "Hello world",
		})
	})
}
