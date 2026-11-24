package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	daemonpb "encr.dev/proto/encore/daemon"
)

// TestRunCurlHTTPErrors verifies that a curl command fails when the app
// responds with an HTTP error status, and still prints the response, unless
// the command passes --no-fail.
func TestRunCurlHTTPErrors(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl not found on PATH")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("ok-body"))
		case "/fail":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("fail-body"))
		case "/redirect":
			http.Redirect(w, r, "/missing", http.StatusFound)
		default:
			http.Error(w, "missing-body", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	tests := []struct {
		name     string
		path     string
		args     []string
		wantOK   bool
		wantBody string
	}{
		{"success", "/ok", nil, true, "ok-body"},
		{"not found", "/missing", nil, false, "missing-body"},
		{"server error", "/fail", nil, false, "fail-body"},
		{"opt out with --no-fail", "/missing", []string{"--no-fail"}, true, "missing-body"},
		{"user passes -f", "/missing", []string{"-f"}, false, ""},
		{"user passes -sSf", "/missing", []string{"-sSf"}, false, ""},
		{"user passes --fail-with-body", "/missing", []string{"--fail-with-body"}, false, "missing-body"},
		{"user writes the status code", "/missing", []string{"-o", os.DevNull, "-w", "%{http_code}"}, false, "404"},
		{"option value that looks like a flag", "/missing", []string{"--user-agent", "--fail"}, false, "missing-body"},
		{"attached option value", "/fail", []string{"-dfoo=bar"}, false, "fail-body"},
		{"redirect not followed", "/redirect", nil, true, ""},
		{"redirect followed to an error", "/redirect", []string{"-L"}, false, "missing-body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := runCurl(context.Background(), addr, &daemonpb.CurlCommand{Path: tt.path, Args: tt.args})
			if gotOK := exitCode == 0; gotOK != tt.wantOK {
				t.Errorf("exit code = %d, want success = %v (stderr: %q)", exitCode, tt.wantOK, stderr)
			}
			if exitCode == 2 {
				t.Errorf("curl rejected its arguments (stderr: %q)", stderr)
			}
			if tt.wantBody != "" && !strings.Contains(string(stdout), tt.wantBody) {
				t.Errorf("stdout = %q, want it to contain %q", stdout, tt.wantBody)
			}
		})
	}
}

func TestLastHTTPStatus(t *testing.T) {
	tests := []struct {
		dump string
		want int
	}{
		{"", 0},
		{"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\n", 200},
		{"HTTP/2 404 \r\ncontent-type: text/plain\r\n\r\n", 404},
		{"HTTP/1.1 302 Found\r\nLocation: /x\r\n\r\nHTTP/1.1 500 Internal Server Error\r\n\r\n", 500},
		{"HTTP/1.1 100 Continue\r\n\r\nHTTP/1.1 201 Created\r\n\r\n", 201},
		{"X-Note: HTTP/1.1 500\r\n", 0},
	}
	for _, tt := range tests {
		if got := lastHTTPStatus([]byte(tt.dump)); got != tt.want {
			t.Errorf("lastHTTPStatus(%q) = %d, want %d", tt.dump, got, tt.want)
		}
	}
}

func TestWaitForHealthz(t *testing.T) {
	statusServer := func(code int) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("an app that exits stops the wait at once", func(t *testing.T) {
		srv := statusServer(http.StatusBadGateway)
		exited := make(chan struct{})
		time.AfterFunc(100*time.Millisecond, func() { close(exited) })
		start := time.Now()
		err := waitForHealthz(context.Background(), srv.URL, 30*time.Second, exited)
		if !errors.Is(err, errAppExited) {
			t.Fatalf("got %v, want errAppExited", err)
		}
		if waited := time.Since(start); waited > 5*time.Second {
			t.Errorf("waited %v after the app exited", waited)
		}
	})

	t.Run("an app that already exited fails at once", func(t *testing.T) {
		srv := statusServer(http.StatusOK)
		exited := make(chan struct{})
		close(exited)
		if err := waitForHealthz(context.Background(), srv.URL, 30*time.Second, exited); !errors.Is(err, errAppExited) {
			t.Fatalf("got %v, want errAppExited", err)
		}
	})

	t.Run("a healthy app is ready", func(t *testing.T) {
		srv := statusServer(http.StatusOK)
		if err := waitForHealthz(context.Background(), srv.URL, 5*time.Second, make(chan struct{})); err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	})

	t.Run("an app that never gets healthy times out", func(t *testing.T) {
		srv := statusServer(http.StatusBadGateway)
		err := waitForHealthz(context.Background(), srv.URL, 300*time.Millisecond, make(chan struct{}))
		if err == nil || err.Error() != "status 502" {
			t.Fatalf("got %v, want the last status, status 502", err)
		}
	})
}

func TestHealthzFailure(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		firstErr string
		want     string
	}{
		{
			name: "exited",
			err:  errAppExited,
			want: "app exited before it was ready",
		},
		{
			name:     "exited with an error logged",
			err:      errAppExited,
			firstErr: "orders: service initialization failed",
			want:     "app exited before it was ready (first error the app logged: orders: service initialization failed)",
		},
		{
			name:     "timed out with an error logged",
			err:      errors.New("status 502"),
			firstErr: "orders: service initialization failed",
			want:     "timed out waiting for /__encore/healthz: status 502 (first error the app logged: orders: service initialization failed)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthzFailure(tt.err, tt.firstErr); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrorLogLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "error line",
			in:   `{"level":"error","error":"dial tcp 127.0.0.1:5432: connect: connection refused","service":"orders","message":"service initialization failed"}`,
			want: "orders: service initialization failed: dial tcp 127.0.0.1:5432: connect: connection refused",
		},
		{name: "error line without a service", in: "{\"level\":\"fatal\",\"message\":\"a fatal error occurred\"}\n", want: "a fatal error occurred"},
		{name: "error that is not a string", in: `{"level":"error","message":"request failed","error":{"code":"internal"}}`, want: `request failed: {"code":"internal"}`},
		{name: "warning", in: `{"level":"warn","message":"broker terminated abruptly"}`},
		{name: "plain text", in: "encore: waiting for app to start (timeout 5m0s)...\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errorLogLine([]byte(tt.in)); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunSpecSink_FirstError(t *testing.T) {
	sink := newRunSpecSink(discardRunSpecStream{})
	w := sink.Stdout(false)
	write := func(line string) {
		t.Helper()
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
	}

	// The app's log writer sends one line per write.
	write(`{"level":"info","message":"registered subscription"}`)
	write(`{"level":"error","service":"orders","message":"service initialization failed"}`)
	write(`{"level":"error","message":"a fatal error occurred, initiating graceful shutdown"}`)
	if got, want := sink.FirstError(), "orders: service initialization failed"; got != want {
		t.Errorf("FirstError() = %q, want %q", got, want)
	}

	// Errors logged once the app is ready are not captured.
	sink = newRunSpecSink(discardRunSpecStream{})
	w = sink.Stdout(false)
	sink.StopCapturingErrors()
	write(`{"level":"error","service":"orders","message":"request failed"}`)
	if got := sink.FirstError(); got != "" {
		t.Errorf("FirstError() after StopCapturingErrors = %q, want \"\"", got)
	}
}

// discardRunSpecStream drops what the sink sends. It embeds the stream interface for its method set;
// the sink calls only Send.
type discardRunSpecStream struct {
	daemonpb.Daemon_RunSpecServer
}

func (discardRunSpecStream) Send(*daemonpb.RunSpecMessage) error { return nil }
