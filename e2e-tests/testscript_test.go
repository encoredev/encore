package tests

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nsqio/go-nsq"
	ts "github.com/rogpeppe/go-internal/testscript"

	"encr.dev/pkg/golden"
)

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
				if len(args) < 2 {
					ts.Fatalf("usage: call <method> <url> [data]")
				} else if len(args) > 3 {
					ts.Fatalf("usage: call <method> <url> [data]")
				}

				app := ts.Value("app").(*AppTestData)
				var body io.Reader
				if len(args) == 3 {
					body = strings.NewReader(args[2])
				}

				req, err := http.NewRequest(args[0], "http://"+app.Addr+args[1], body)
				if err != nil {
					ts.Fatalf("unable to create request: %v", err)
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					ts.Fatalf("unable to make request: %v", err)
				}
				io.Copy(os.Stdout, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK && !neg {
					ts.Fatalf("unexpected status code: %v", resp.StatusCode)
				} else if resp.StatusCode == http.StatusOK && neg {
					ts.Fatalf("unexpected status code: %v", resp.StatusCode)
				}
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

				var want []logLine
				var pattern logLine
				err := json.Unmarshal([]byte(args[0]), &pattern)
				if err != nil {
					if strings.Contains(args[0], "{") {
						ts.Fatalf("checklog pattern not valid log line: %v", err)
					}

					fn := ts.ReadFile(args[0])
					scanner := bufio.NewScanner(strings.NewReader(fn))
					for scanner.Scan() {
						var ln logLine
						if err := json.Unmarshal(scanner.Bytes(), &ln); err != nil {
							ts.Fatalf("invalid log line in checklog script: %s: %v", scanner.Bytes(), err)
						}
						want = append(want, ln)
					}
				} else {
					want = []logLine{pattern}
				}

				log := ts.Value("log").(*testBufferLogger)
				stderr := log.stderr.String()
				scanner := bufio.NewScanner(strings.NewReader(stderr))

				seen := make([]bool, len(want))
				for scanner.Scan() {
					var got logLine
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

type logLine = map[string]any

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
