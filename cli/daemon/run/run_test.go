package run

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/hashicorp/yamux"
	"go.uber.org/goleak"

	"encr.dev/cli/internal/codegen"
	"encr.dev/cli/internal/env"
	"encr.dev/compiler"
	"encr.dev/pkg/golden"
)

type Data[K any, V any] struct {
	Key   K
	Value V
}

type NonBasicRequest struct {
	Struct        Data[*Data[string, string], int]
	StructPtr     *Data[int, uint16]
	StructSlice   []*Data[string, string]
	StructMap     map[string]*Data[string, float32]
	StructMapPtr  *map[string]*Data[string, string]
	AnonStruct    struct{ AnonBird string }
	NamedStruct   *Data[string, float64] `json:"formatted_nest"`
	RawStruct     Data[[]string, []byte]
	UnusedRequest *NonBasicRequest
}

type NonBasicResponse struct {
	// Body
	Struct       Data[*Data[string, string], int]
	StructPtr    *Data[int, uint16]
	StructSlice  []*Data[string, string]
	StructMap    map[string]*Data[string, float32]
	StructMapPtr *map[string]*Data[string, string]
	AnonStruct   struct{ AnonBird string }
	NamedStruct  *Data[string, float64] `json:"formatted_nest"`
	RawStruct    json.RawMessage

	// Query
	QueryString string
	QueryNumber int

	// Path
	PathString string
	PathInt    int
	PathWild   string
}

func TestMain(m *testing.M) {
	golden.TestMain(m)
}

// TestEndToEndWithApp tests that (*app).startProc correctly starts Encore processes
// for sending requests.
func TestEndToEndWithApp(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	c := qt.New(t)

	ln, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, qt.IsNil)
	defer ln.Close()

	run := &Run{ID: genID(), ListenAddr: ln.Addr().String(), AppSlug: "slug"}
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

	// start proxying TCP requests to the running application
	go proxyTcp(ctx, ln, p.client)

	// Use golden to test that the generated clients are as expected for the echo test app
	for lang, path := range map[codegen.Lang]string{codegen.LangGo: "client/client.go", codegen.LangTypeScript: "client.ts"} {
		client, err := codegen.Client(lang, "slug", build.Parse.Meta)
		c.Assert(err, qt.IsNil, qt.Commentf("Got an error generating the client for: %s", lang))

		golden.TestAgainst(c, filepath.Join("echo_client", path), string(client))
	}

	c.Run("basic requests", func(c *qt.C) {
		// Send a simple request
		{
			input := Data[string, int]{"hello", 1}
			body, err := json.Marshal(&input)
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/echo.Echo", bytes.NewReader(body))
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, input)
		}

		// Call an endpoint using an unsupported HTTP Method
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Echo", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 404)
		}

		// Send an empty request
		{
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/echo.EmptyEcho", bytes.NewReader([]byte("{}")))
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{
				"NullPtr": nil,
				"Zero":    Data[string, string]{},
			})
		}

		// Send a non-basic type request with path params, headers, query string and body
		{
			input := NonBasicRequest{
				Struct:      Data[*Data[string, string], int]{&Data[string, string]{"peacock", "duck"}, 1},
				StructPtr:   &Data[int, uint16]{2, 3},
				StructSlice: []*Data[string, string]{{"seagull", "penguin"}, {"penguin", "seagull"}},
				StructMap: map[string]*Data[string, float32]{
					"hawk":      {"hummingbird", 18.5},
					"albatross": {"magpie", 13.2},
				},
				StructMapPtr: &map[string]*Data[string, string]{
					"hornbill": {"bird-of-paradise", "cuckoo"},
					"turkey":   {"owl", "waxbill"},
				},
				AnonStruct:    struct{ AnonBird string }{AnonBird: "dove"},
				NamedStruct:   &Data[string, float64]{"pigeon", 34.2},
				UnusedRequest: &NonBasicRequest{StructPtr: &Data[int, uint16]{43, 9}},
				RawStruct:     Data[[]string, []byte]{[]string{"emu", "ostrich"}, []byte{4, 4, 5}},
			}

			output := NonBasicResponse{
				Struct:       input.Struct,
				StructPtr:    input.StructPtr,
				StructSlice:  input.StructSlice,
				StructMap:    input.StructMap,
				StructMapPtr: input.StructMapPtr,
				AnonStruct:   input.AnonStruct,
				NamedStruct:  input.NamedStruct,
				QueryString:  "robin",
				QueryNumber:  33,
				RawStruct:    json.RawMessage(`{"Key": ["emu", "ostrich"], "Value": "BAQF"}`),
				PathString:   "shoebill",
				PathInt:      55,
				PathWild:     "toucan/crane/vulture/78/",
			}
			body, err := json.Marshal(&input)
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/NonBasicEcho/shoebill/55/toucan/crane/vulture/78/?string=robin&no=33", bytes.NewReader(body))
			req.Header.Add("X-Header-String", "starling")
			req.Header.Add("X-Header-Number", "10")
			req.Header.Add("Authorization", "Bearer tokendata")
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Header().Get("X-Header-String"), qt.Equals, "starling")
			c.Assert(w.Header().Get("X-Header-Number"), qt.Equals, "10")
			c.Assert(w.Body.Bytes(), qt.JSONEquals, output)
		}

		// Send a request with only header parameters
		{
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.HeadersEcho", nil)
			req.Header.Add("x-int", "1")
			req.Header.Add("x-string", "nightingale")
			req.Header.Add("X-StringSlice", "mynah, quail, weaver")
			req.Header.Add("X-StringSlice", "pewit")
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Header().Get("X-Int"), qt.Equals, "1")
			c.Assert(w.Header().Get("X-String"), qt.Equals, "nightingale")
		}

		// Send POST and GET requests to the same endpoint with an assortment of basic types
		{
			input := map[string]any{
				"string":       "string",
				"uint":         1,
				"int":          2,
				"int8":         -3,
				"int64":        4,
				"float32":      5,
				"float64":      6,
				"string_slice": []any{"slice1", "slice2"},
				"int_slice":    []any{1, 2, 3},
				"time":         "2016-01-02T15:04:05+07:00",
			}
			output := map[string]any{
				"String":      "string",
				"Uint":        1,
				"Int":         2,
				"Int8":        -3,
				"Int64":       4,
				"Float32":     5,
				"Float64":     6,
				"StringSlice": []any{"slice1", "slice2"},
				"IntSlice":    []any{1, 2, 3},
				"Time":        "2016-01-02T15:04:05+07:00",
			}
			qs := ""
			for k, av := range input {
				vs := []any{av}
				switch av.(type) {
				case []any:
					vs = av.([]any)
				}
				for _, v := range vs {
					if len(qs) > 0 {
						qs += "&"
					}
					value := url.QueryEscape(fmt.Sprintf("%v", v))
					qs += fmt.Sprintf("%s=%v", strings.ToLower(k), value)
				}
			}
			body, err := json.Marshal(&output)
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			w2 := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.BasicEcho?"+qs, nil)
			req2 := httptest.NewRequest("POST", "/echo.BasicEcho", bytes.NewReader(body))
			run.ServeHTTP(w, req)
			run.ServeHTTP(w2, req2)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, output)
			c.Assert(w2.Body.Bytes(), qt.DeepEquals, w2.Body.Bytes())
		}

		// Call an endpoint without request parameters and response value
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Noop", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
		}

		// Call an endpoint with request parameters but no response value
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.MuteEcho?key=pelican&value=cocabura", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
		}

		// Call an endpoint with a response value but no request parameters
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Pong", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, Data[string, string]{"woodpecker", "kingfisher"})
		}

		// Call the env endpoint and make sure we get our env variables back
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Env", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string][]string{"Env": wantEnv})
		}

		// Call the app metadata endpoint and make sure we get correct data back
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.AppMeta", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)

			// we need to extract the API Base URL as it will change due to the `RuntimePort: 0` above
			bytes := w.Body.Bytes()
			got := make(map[string]string)
			_ = json.Unmarshal(bytes, &got)

			c.Assert(strings.HasPrefix(got["APIBaseURL"], "http://"), qt.IsTrue)
			c.Assert(bytes, qt.JSONEquals, map[string]interface{}{
				"AppID":      "slug",
				"APIBaseURL": got["APIBaseURL"],
				"EnvName":    "local",
				"EnvType":    "local",
			})
		}
	})

	c.Run("go_generated_client", func(c *qt.C) {
		cmd := exec.Command("go", "run", ".", ln.Addr().String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Dir = filepath.Join("testdata", "echo_client")

		c.Assert(cmd.Run(), qt.IsNil, qt.Commentf("Got error running generated Go client"))
	})

	c.Run("typescript_generated_client", func(c *qt.C) {
		npmCommandsToRun := [][]string{
			{"install", "--prefer-offline", "--no-audit"},
			{"run", "lint"},
			{"run", "test", "--", ln.Addr().String()},
		}

		for _, args := range npmCommandsToRun {
			cmd := exec.Command("npm", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Dir = filepath.Join("testdata", "echo_client")

			c.Assert(cmd.Run(), qt.IsNil, qt.Commentf("Got error running generated Typescript client"))
		}
	})
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
		Environ:     os.Environ(),
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
		BuildTags:         []string{"encore_local"},
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

func proxyTcp(ctx context.Context, ln net.Listener, client *yamux.Session) {
	for ctx.Err() == nil {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			fmt.Printf("unable to accept connection: %+v", err)
			continue
		}

		clientConn, err := client.Open()
		if err != nil {
			fmt.Printf("unable to open connection to running app: %+v", err)
			_ = conn.Close()
			continue
		}

		go io.Copy(conn, clientConn)
		go io.Copy(clientConn, conn)
	}
}
