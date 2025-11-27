//go:build e2e

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/rs/zerolog/log"
	"go.uber.org/goleak"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	. "encr.dev/cli/daemon/run"
	"encr.dev/cli/daemon/run/infra"
	. "encr.dev/internal/optracker"
	"encr.dev/pkg/clientgen"
	"encr.dev/pkg/clientgen/clientgentypes"
	"encr.dev/pkg/golden"
	"encr.dev/pkg/svcproxy"
	"encr.dev/v2/v2builder"
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
	QueryString    string
	QueryNumber    int
	OptQueryString string
	OptQueryNumber int

	// Path
	PathString string
	PathInt    int
	PathWild   string

	// Auth
	AuthHeader string
	AuthQuery  []int
}

// TestEndToEndWithApp tests that (*app).startProc correctly starts Encore processes
// for sending requests.
func TestEndToEndWithApp(t *testing.T) {
	doTestEndToEndWithApp(t, nil)
}

func doTestEndToEndWithApp(t *testing.T, env []string) {
	c := qt.New(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(wd, "testdata", "echo")
	app := RunApp(c, appRoot, nil, env)
	run := app.Run

	// Use golden to test that the generated clients are as expected for the echo test app
	for lang, path := range map[clientgen.Lang]string{
		clientgen.LangGo:         "golang/client/goclient.go",
		clientgen.LangTypeScript: "ts/client.ts",
		clientgen.LangJavascript: "js/client.js",
	} {
		services := clientgentypes.AllServices(app.Meta)
		client, err := clientgen.Client(lang, "slug", app.Meta, services, clientgentypes.TagSet{}, clientgentypes.Options{})
		if err != nil {
			fmt.Println(err.Error())
			c.FailNow()
		}
		c.Assert(err, qt.IsNil, qt.Commentf("Got an error generating the client for: %s", lang))

		golden.TestAgainst(c, filepath.Join("echo_client", path), string(client))
	}

	c.Run("basic requests", func(c *qt.C) {
		// Send a simple request
		c.Run("Send a simple request", func(c *qt.C) {
			input := Data[string, int]{"hello", 1}
			body, err := json.Marshal(&input)
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/echo.Echo", bytes.NewReader(body))
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, input)
		})

		// Send a pubsub
		c.Run("send a pubsub", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/echo.Publish", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)

			// Wait a bit to allow the message to be consumed.
			time.Sleep(100 * time.Millisecond)

			stats, err := app.NSQ.Stats()
			c.Assert(err, qt.IsNil)
			c.Assert(len(stats.Producers), qt.Equals, 1)
			c.Assert(len(stats.Topics), qt.Equals, 1)
			c.Assert(stats.Topics[0].TopicName, qt.Equals, "test")
			c.Assert(len(stats.Topics[0].Channels), qt.Equals, 1)
			c.Assert(stats.Topics[0].Channels[0].RequeueCount == 0, qt.IsTrue)
			c.Assert(stats.Topics[0].Channels[0].Depth == 0, qt.IsTrue)
		})

		// Call an endpoint using an unsupported HTTP Method
		c.Run("unsupported HTTP Method", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Echo", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 404)
		})

		// Send an empty request
		c.Run("empty request", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/echo.EmptyEcho", bytes.NewReader([]byte("{}")))
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{
				"NullPtr": nil,
				"Zero":    Data[string, string]{},
			})
		})

		// Send a non-basic type request with path params, headers, query string and body
		c.Run("non-basic type request", func(c *qt.C) {
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
				AuthHeader:   "header",
				AuthQuery:    []int{5, 6},
			}
			body, err := json.Marshal(&input)
			c.Assert(err, qt.IsNil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/NonBasicEcho/shoebill/55/toucan/crane/vulture/78/?string=robin&no=33&query=5&query=6", bytes.NewReader(body))
			req.Header.Add("X-Header", "header")
			req.Header.Add("X-Header-String", "starling")
			req.Header.Add("X-Header-Number", "10")
			req.Header.Add("Authorization", "Bearer tokendata")
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Header().Get("X-Header-String"), qt.Equals, "starling")
			c.Assert(w.Header().Get("X-Header-Number"), qt.Equals, "10")
			c.Assert(w.Body.Bytes(), qt.JSONEquals, output)
		})

		// Send a request with only header parameters
		c.Run("only headers", func(c *qt.C) {
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
		})

		// Send POST and GET requests to the same endpoint with an assortment of basic types
		c.Run("POST and GET", func(c *qt.C) {
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
			c.Assert(w2.Body.Bytes(), qt.DeepEquals, w.Body.Bytes())
		})

		// Call an endpoint without request parameters, returning nil
		c.Run("without request", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.NilResponse", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
		})

		// Call an endpoint with an invalid auth parameter
		c.Run("invalid parameter", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.NilResponse", nil)
			req.Header.Add("x-auth-int", "invalid")
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 400)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{
				"code":    "invalid_argument",
				"details": nil,
				"message": "invalid auth param: x-auth-int: invalid parameter: strconv.ParseInt: parsing \"invalid\": invalid syntax",
			})
		})

		// Call an endpoint without request parameters and response value
		c.Run("without response", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Noop", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
		})

		// Call an endpoint with request parameters but no response value
		c.Run("only request", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.MuteEcho?key=pelican&value=cocabura", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
		})

		// Call an endpoint with a response value but no request parameters
		c.Run("no request, response", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Pong", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, Data[string, string]{"woodpecker", "kingfisher"})
		})

		// Call endpoint with custom http status in response
		c.Run("custom http status response", func(c *qt.C) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.CustomHTTPStatus", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 201)
		})

		// Call the env endpoint and make sure we get our env variables back
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.Env", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)

			filteredEnv := make([]string, 0, len(app.Env))
			for _, env := range app.Env {
				if !strings.HasPrefix(env, "ENCORE_") {
					filteredEnv = append(filteredEnv, env)
				}
			}
			c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string][]string{"Env": filteredEnv})
		}

		// Call the app metadata endpoint and make sure we get correct data back
		c.Run("app metadata", func(c *qt.C) {
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
				"AppID":      "",
				"APIBaseURL": got["APIBaseURL"],
				"EnvName":    "local",
				"EnvType":    "development",
			})
		})

		// Try the dependency injection services
		c.Run("dependency_injection", func(c *qt.C) {
			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/di/one", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.HasLen, 0)
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/di/two", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]string{"Msg": "Hello World"})
			}

		})

		c.Run("cache", func(c *qt.C) {
			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/incr/one", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Val": 1})
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/incr/one", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Val": 2})
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/incr/two", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Val": 1})
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("POST", "/cache/struct/1/foo", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.HasLen, 0)
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/struct/1", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Val": "foo"})
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/list/1", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Vals": []string{}})
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("POST", "/cache/list/1/foo", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.HasLen, 0)
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/list/1", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Vals": []string{"foo"}})
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("POST", "/cache/list/1/bar", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.HasLen, 0)
			}

			{
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/cache/list/1", nil)
				run.ServeHTTP(w, req)
				c.Assert(w.Code, qt.Equals, 200)
				c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{"Vals": []string{"foo", "bar"}})
			}

			keys := app.Redis.Miniredis().Keys()
			c.Assert(keys, qt.DeepEquals, []string{
				"int/one",
				"int/two",
				"list/1/foo/1",
				"struct/1/dummy/x",
			})
		})
	})

	c.Run("generated_wrappers_for_intra_service_calls", func(c *qt.C) {
		// Send a simple request
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/generated-wrappers-end-to-end-test", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
		}
	})

	c.Run("config_test", func(c *qt.C) {
		{
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/echo.ConfigValues", nil)
			run.ServeHTTP(w, req)
			c.Assert(w.Code, qt.Equals, 200)
			c.Assert(w.Body.Bytes(), qt.JSONEquals, map[string]any{
				"ReadOnlyMode": true,
				"PublicKey":    "aGVsbG8gd29ybGQK",
				"AdminUsers":   []string{"foo", "bar"},
				"SubKeyCount":  2,
			})
		}
	})

	c.Run("go_generated_client", func(c *qt.C) {
		encoreGoroot := os.Getenv("ENCORE_GOROOT")
		c.Assert(encoreGoroot, qt.Not(qt.Equals), "")
		goPath := filepath.Join(encoreGoroot, "bin", "go")
		cmd := exec.Command(goPath, "run", ".", app.Addr)
		cmd.Dir = filepath.Join("testdata", "echo_client")
		cmd.Env = append(os.Environ(),
			"GOROOT="+encoreGoroot,
			"PATH="+fmt.Sprintf("%s%s%s", filepath.Join(encoreGoroot, "/bin"), string(filepath.ListSeparator), os.Getenv("PATH")),
		)

		out, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil, qt.Commentf("Got error running generated Go client: %s", out))
	})

	c.Run("typescript_generated_client", func(c *qt.C) {
		npmCommandsToRun := [][]string{
			{"install", "--prefer-offline", "--no-audit"},
			{"run", "lint"},
			{"run", "test", "--", app.Addr},
		}

		for _, args := range npmCommandsToRun {
			cmd := exec.Command("npm", args...)
			cmd.Dir = filepath.Join("testdata", "echo_client")

			out, err := cmd.CombinedOutput()
			c.Assert(err, qt.IsNil, qt.Commentf("Got error running generated Typescript client: %s", out))
		}
	})

	c.Run("javascript_generated_client", func(c *qt.C) {
		npmCommandsToRun := [][]string{
			{"install", "--prefer-offline", "--no-audit"},
			{"run", "lint"},
			{"run", "test:js", "--", app.Addr},
		}

		for _, args := range npmCommandsToRun {
			cmd := exec.Command("npm", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Dir = filepath.Join("testdata", "echo_client")

			c.Assert(cmd.Run(), qt.IsNil, qt.Commentf("Got error running generated JavaScript client"))
		}
	})
}

// TestProcClosedOnCtxCancel tests that the proc is closed when
// the given ctx is cancelled.
func TestProcClosedOnCtxCancel(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	appRoot := filepath.Join(wd, "testdata", "echo")

	app := apps.NewInstance(appRoot, "local_id", "platform_id")

	c := qt.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svcProxy, err := svcproxy.New(ctx, log.Logger)
	c.Assert(err, qt.IsNil)

	mgr := &Manager{}
	ns := &namespace.Namespace{ID: "some-id", Name: "default"}
	rm := infra.NewResourceManager(app, nil, nil, nil, ns, nil, 0, false)
	run := &Run{
		ID:              GenID(),
		App:             app,
		Mgr:             mgr,
		ResourceManager: rm,
		ListenAddr:      "127.0.0.1:34212",
		SvcProxy:        svcProxy,
		Builder:         v2builder.New(),
		Params:          &StartParams{},
	}

	parse, build, _ := testBuild(c, appRoot, append(os.Environ(), "ENCORE_EXPERIMENT=v2"))
	jobs := NewAsyncBuildJobs(ctx, app.PlatformOrLocalID(), nil)
	run.ResourceManager.StartRequiredServices(jobs, parse.Meta)
	defer run.Close()

	c.Assert(jobs.Wait(), qt.IsNil)

	p, err := run.StartProcGroup(&StartProcGroupParams{
		Ctx:     ctx,
		Outputs: build.Outputs,
		Meta:    parse.Meta,
		Logger:  testRunLogger{t},
		Environ: os.Environ(),
	})
	c.Assert(err, qt.IsNil)
	cancel()
	<-p.Done()
}
