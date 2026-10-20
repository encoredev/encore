package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

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
