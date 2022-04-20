package run

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"go.uber.org/goleak"

	"encr.dev/cli/internal/env"
	"encr.dev/compiler"
)

// TestStartProc tests that (*app).startProc correctly starts Encore processes
// for sending requests.
func TestStartProc(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	c := qt.New(t)

	ln, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, qt.IsNil)
	defer ln.Close()

	run := &Run{ID: genID(), ListenAddr: ln.Addr().String()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	build := testBuild(c, "./testdata/echo")
	wantEnv := []string{"FOO=bar", "BAR=baz"}
	p, err := run.startProc(&startProcParams{
		Ctx:         ctx,
		BuildDir:    build.Dir,
		BinPath:     build.Exe,
		Meta:        build.Parse.Meta,
		RuntimePort: 0,
		DBProxyPort: 0,
		Logger:      testRunLogger{t},
		Environ:     wantEnv,
	})
	c.Assert(err, qt.IsNil)
	defer p.close()
	run.proc.Store(p)

	// Send a simple message and make sure it is echoed back.
	{
		input := struct{ Message string }{Message: "hello"}
		body, _ := json.Marshal(&input)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/echo.Echo", bytes.NewReader(body))
		run.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, 200)
		c.Assert(w.Body.Bytes(), qt.JSONEquals, input)
	}

	// Call the env endpoint and make sure we get our env variables back
	{
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/echo.Env", nil)
		run.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, 200)
		c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string][]string{"Env": wantEnv})
	}
}

// TestProcClosedOnCtxCancel tests that the proc is closed when
// the given ctx is cancelled.
func TestProcClosedOnCtxCancel(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	app := &Run{ID: genID()}
	c := qt.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	build := testBuild(c, "./testdata/echo")
	p, err := app.startProc(&startProcParams{
		Ctx:         ctx,
		BuildDir:    build.Dir,
		BinPath:     build.Exe,
		Meta:        build.Parse.Meta,
		RuntimePort: 0,
		DBProxyPort: 0,
		Logger:      testRunLogger{t},
	})
	c.Assert(err, qt.IsNil)
	cancel()
	<-p.Done()
}

// testBuild is a helper that compiles the app situated at appRoot
// and cleans up the build dir during test cleanup.
func testBuild(c *qt.C, appRoot string) *compiler.Result {
	wd, err := os.Getwd()
	c.Assert(err, qt.IsNil)
	runtimePath := filepath.Join(wd, "../../../runtime")
	build, err := compiler.Build("./testdata/echo", &compiler.Config{
		EncoreRuntimePath: runtimePath,
		EncoreGoRoot:      env.EncoreGoRoot(),
	})
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() {
		os.RemoveAll(build.Dir)
	})
	return build
}

// testRunLogger implements runLogger by calling t.Log.
type testRunLogger struct {
	t *testing.T
}

func (l testRunLogger) runStdout(r *Run, line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	l.t.Log(string(line))
}

func (l testRunLogger) runStderr(r *Run, line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\n'})
	l.t.Log(string(line))
}
