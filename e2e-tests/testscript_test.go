package tests

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nsqio/go-nsq"
	ts "github.com/rogpeppe/go-internal/testscript"

	"encr.dev/cli/daemon/run"
	"encr.dev/pkg/golden"
)

// headerRe matches valid headers in the form "Header=value".
var headerRe = regexp.MustCompile(`^([A-Z][A-Za-z0-9-]*)=([^ ]*)$`)

func TestRun(t *testing.T) {
	runtimePath := os.Getenv("ENCORE_RUNTIME_PATH")
	goroot := os.Getenv("ENCORE_GOROOT")
	if testing.Short() {
		t.Skip("skipping in short mode")
	} else if runtimePath == "" || goroot == "" {
		t.Skipf("skipping due to missing ENCORE_RUNTIME_PATH=%q or ENCORE_GOROOT=%q", runtimePath, goroot)
	}

	home := t.TempDir()

	ts.Run(t, ts.Params{
		Dir: "testdata/testscript",
		Setup: func(e *ts.Env) error {
			e.Setenv("ENCORE_RUNTIME_PATH", runtimePath)
			e.Setenv("ENCORE_GOROOT", goroot)
			e.Setenv("HOME", home)
			e.Setenv("GOFLAGS", "-modcacherw")
			gomod := []byte("module test\n\nrequire encore.dev v1.9.4")
			if err := os.WriteFile(filepath.Join(e.WorkDir, "go.mod"), gomod, 0755); err != nil {
				return err
			}

			log := &testBufferLogger{tb: e.T().(testing.TB)}
			app := AppTest(e.T().(testing.TB), e.WorkDir, log)
			e.Values["app"] = app
			e.Values["log"] = log
			return nil
		},
		Cmds: map[string]func(ts *ts.TestScript, neg bool, args []string){
			"call": func(ts *ts.TestScript, neg bool, args []string) {
				usage := func() {
					ts.Fatalf("usage: call <method> [Header=value...] <url> [data] [platform-auth]")
				}
				if len(args) < 2 {
					usage()
				}

				method := args[0]
				args = args[1:]
				headers := make(map[string]string)
				for i, arg := range args {
					m := headerRe.FindStringSubmatch(arg)
					if m != nil {
						headers[m[1]] = m[2]
					} else {
						args = args[i:]
						break
					}
				}

				if len(args) == 0 {
					usage()
				}
				app := ts.Value("app").(*AppTestData)
				url := "http://" + app.Addr + args[0]

				disablePlatformAuth := false

				var body io.Reader
				if n := len(args); n > 1 {
					val := args[1]
					if val == "no-platform-auth" {
						disablePlatformAuth = true
					} else {
						body = strings.NewReader(val)
						if n > 2 {
							if args[2] == "no-platform-auth" {
								disablePlatformAuth = true
							} else {
								ts.Fatalf("unexpected argument %q", args[2])
							}
						}
					}
				}

				req := httptest.NewRequest(method, url, body)
				for k, v := range headers {
					ts.Logf("setting %s=%v", k, v)
					req.Header.Set(k, v)
				}

				if disablePlatformAuth {
					req.Header.Set(run.TestHeaderDisablePlatformAuth, "true")
				}

				w := httptest.NewRecorder()
				app.Run.ServeHTTP(w, req)
				respBody := w.Body.Bytes()
				os.Stdout.Write(respBody)

				if w.Code != http.StatusOK && !neg {
					ts.Fatalf("unexpected status code: %v", w.Code)
				} else if w.Code == http.StatusOK && neg {
					ts.Fatalf("unexpected status code: %v", w.Code)
				}
				app.Values["call_resp"] = respBody
			},
			"publish": func(ts *ts.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: publish <topic> <data>")
				}
				topicName := args[0]
				data := args[1]

				app := ts.Value("app").(*AppTestData)

				id, _ := app.Values["publish_id"].(int)
				id++
				app.Values["publish_id"] = id
				msgData, _ := json.Marshal(messageWrapper{
					ID:         strconv.Itoa(id),
					Attributes: nil,
					Data:       json.RawMessage(data),
				})

				prod, err := nsq.NewProducer(app.NSQ.Addr(), nsq.NewConfig())
				if err != nil {
					ts.Fatalf("unable to create producer: %v", err)
				}
				prod.SetLoggerLevel(nsq.LogLevelMax)
				if err := prod.Publish(topicName, msgData); err != nil {
					ts.Fatalf("unable to publish: %v", err)
				}

				// Wait for the message to get processed
				for i := 0; i < 100; i++ {
					stats, err := app.NSQ.Stats()
					if err != nil {
						ts.Fatalf("unable to get nsq stats: %v", err)
					}
					for _, topic := range stats.Topics {
						if topic.TopicName == topicName {
							if topic.Depth == 0 {
								break
							}
							ts.Logf("waiting for queue to be processed, depth: %d", topic.Depth)
							time.Sleep(100 * time.Millisecond)
						}
					}
				}
			},
			"checklog": func(ts *ts.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: checklog <pattern|file>")
				}

				var want []jsonObj
				var pattern jsonObj
				err := json.Unmarshal([]byte(args[0]), &pattern)
				if err != nil {
					if strings.Contains(args[0], "{") {
						ts.Fatalf("checklog pattern not valid log line: %v", err)
					}

					fn := ts.ReadFile(args[0])
					scanner := bufio.NewScanner(strings.NewReader(fn))
					for scanner.Scan() {
						var ln jsonObj
						if err := json.Unmarshal(scanner.Bytes(), &ln); err != nil {
							ts.Fatalf("invalid log line in checklog script: %s: %v", scanner.Bytes(), err)
						}
						want = append(want, ln)
					}
				} else {
					want = []jsonObj{pattern}
				}

				log := ts.Value("log").(*testBufferLogger)
				stderr := log.stderr.String()
				scanner := bufio.NewScanner(strings.NewReader(stderr))

				seen := make([]bool, len(want))
				for scanner.Scan() {
					var got jsonObj
					if err := json.Unmarshal(scanner.Bytes(), &got); err == nil {
						for i, ln := range want {
							if !seen[i] && gotLogLine(got, ln) {
								seen[i] = true
							}
						}
					}
				}

				for i, ln := range want {
					if !neg && !seen[i] {
						ts.Fatalf("unable to find log line: %v", ln)
					} else if neg && seen[i] {
						ts.Fatalf("found log line: %v", ln)
					}
				}
			},
			"checkresp": func(ts *ts.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: checkresp <pattern|file>")
				}

				var want jsonObj
				err := json.Unmarshal([]byte(args[0]), &want)
				if err != nil {
					if strings.Contains(args[0], "{") {
						ts.Fatalf("checkresp pattern not valid log line: %v", err)
					}
					fn := ts.ReadFile(args[0])
					if err := json.Unmarshal([]byte(fn), &want); err != nil {
						ts.Fatalf("invalid json object in checkresp script: %s: %v", fn, err)
					}
				}

				app := ts.Value("app").(*AppTestData)

				var got jsonObj
				if err := json.Unmarshal(app.Values["call_resp"].([]byte), &got); err != nil {
					ts.Fatalf("unable to parse response: %v", err)
				}

				match := gotLogLine(got, want)
				if !neg && !match {
					ts.Fatalf("response does not match: got %s, want %s", got, want)
				} else if neg && match {
					ts.Fatalf("log line unexpectedly matched")
				}
			},
		},
	})
}

func TestMain(m *testing.M) {
	golden.Setup()

	os.Exit(ts.RunMain(m, nil))
}

// messageWrapper is the data structure for an NSQ message.
// It must be synchronized with the nsq/topic.go file in the runtime.
type messageWrapper struct {
	ID         string
	Attributes map[string]string
	Data       json.RawMessage
}

type jsonObj = map[string]any

func gotLogLine(got, want any) bool {
	switch want := want.(type) {
	case map[string]any:
		got, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, v := range want {
			if !gotLogLine(got[k], v) {
				return false
			}
		}
		return true
	default:
		return got == want
	}
}
