//go:build e2e

package tests

import (
	"bufio"
	"bytes"
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
	doRun(t, nil)
}

func doRun(t *testing.T, experiments []string) {
	runtimePath := os.Getenv("ENCORE_RUNTIMES_PATH")
	goroot := os.Getenv("ENCORE_GOROOT")
	if testing.Short() {
		t.Skip("skipping in short mode")
	} else if runtimePath == "" || goroot == "" {
		t.Skipf("skipping due to missing ENCORE_RUNTIMES_PATH=%q or ENCORE_GOROOT=%q", runtimePath, goroot)
	}

	home := t.TempDir()

	ts.Run(t, ts.Params{
		Dir: "testdata/testscript",
		Setup: func(e *ts.Env) error {
			e.Setenv("ENCORE_RUNTIMES_PATH", runtimePath)
			e.Setenv("ENCORE_GOROOT", goroot)
			e.Setenv("EXTRA_EXPERIMENTS", strings.Join(experiments, ","))
			e.Setenv("HOME", home)
			e.Setenv("GOFLAGS", "-modcacherw")
			gomod := []byte("module test\n\ngo 1.21.0\n\nrequire encore.dev v1.52.0")
			if err := os.WriteFile(filepath.Join(e.WorkDir, "go.mod"), gomod, 0755); err != nil {
				return err
			}

			if err := runGoModTidy(e.WorkDir); err != nil {
				return err
			}

			initVals(e)
			return nil
		},
		Cmds: map[string]func(ts *ts.TestScript, neg bool, args []string){
			"run": func(ts *ts.TestScript, neg bool, args []string) {
				log := &testscriptLogger{ts: ts}
				exp := ts.Getenv("ENCORE_EXPERIMENT")
				if extra := ts.Getenv("EXTRA_EXPERIMENTS"); extra != "" {
					if exp != "" {
						exp += ","
					}
					exp += extra
				}

				env := []string{"ENCORE_EXPERIMENT=" + exp}
				if nodePath, ok := getNodeJSPath().Get(); ok {
					env = append(env, "PATH="+nodePath)
				}

				app := RunApp(getTB(ts), getWorkdir(ts), log, env)
				setVal(ts, "app", app)
				setVal(ts, "log", log)
			},
			"shutdown": func(ts *ts.TestScript, neg bool, args []string) {
				app := getVal[*RunAppData](ts, "app")
				app.Run.ProcGroup().Close()
			},
			"test": func(ts *ts.TestScript, neg bool, args []string) {
				log := &testscriptLogger{ts: ts}
				exp := ts.Getenv("ENCORE_EXPERIMENT")
				if extra := ts.Getenv("EXTRA_EXPERIMENTS"); extra != "" {
					if exp != "" {
						exp += ","
					}
					exp += extra
				}

				err := RunTests(getTB(ts), getWorkdir(ts), &log.stdout, &log.stderr, []string{"ENCORE_EXPERIMENT=" + exp})
				_, _ = os.Stdout.Write(log.stdout.Bytes())
				_, _ = os.Stderr.Write(log.stderr.Bytes())
				if !neg && err != nil {
					ts.Fatalf("tests failed: %v", err)
				} else if neg && err == nil {
					ts.Fatalf("tests unexpectedly passed: %v", err)
				}
				setVal(ts, "log", log)
			},
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
				app := getVal[*RunAppData](ts, "app")
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
				_, _ = os.Stdout.Write(respBody)

				if w.Code != http.StatusOK && !neg {
					ts.Fatalf("unexpected status code: %v: %s", w.Code, respBody)
				} else if w.Code == http.StatusOK && neg {
					ts.Fatalf("unexpected status code: %v: %s", w.Code, respBody)
				}
				app.Values["call_resp"] = respBody
			},
			"publish": func(ts *ts.TestScript, neg bool, args []string) {
				if len(args) != 2 {
					ts.Fatalf("usage: publish <topic> <data>")
				}
				topicName := args[0]
				data := args[1]

				app := getVal[*RunAppData](ts, "app")
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
							ts.Logf("waiting for %q queue to be processed, depth: %d", topic.TopicName, topic.Depth)
							time.Sleep(100 * time.Millisecond)
						}
					}
				}
			},
			"checklog": func(ts *ts.TestScript, neg bool, args []string) {
				if len(args) != 1 {
					ts.Fatalf("usage: checklog <pattern|file>")
				}

				time.Sleep(100 * time.Millisecond)

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

				log := getVal[*testscriptLogger](ts, "log")
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

				app := getVal[*RunAppData](ts, "app")

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

type testscriptLogger struct {
	ts             *ts.TestScript
	stdout, stderr bytes.Buffer
}

func (l *testscriptLogger) RunStdout(r *run.Run, line []byte) {
	l.ts.Logf("%s", line)
	l.stdout.Write(line)
}

func (l *testscriptLogger) RunStderr(r *run.Run, line []byte) {
	l.ts.Logf("%s", line)
	l.stderr.Write(line)
}

func initVals(e *ts.Env) {
	e.Values["vars"] = map[string]any{
		"tb": e.T().(testing.TB),
		"wd": e.WorkDir,
	}
}

func getTB(ts *ts.TestScript) testing.TB {
	return getVal[testing.TB](ts, "tb")
}

func getWorkdir(ts *ts.TestScript) string {
	return getVal[string](ts, "wd")
}

func setVal(ts *ts.TestScript, key string, val any) {
	ts.Value("vars").(map[string]any)[key] = val
}

func getVal[T any](ts *ts.TestScript, key string) T {
	return ts.Value("vars").(map[string]any)[key].(T)
}
