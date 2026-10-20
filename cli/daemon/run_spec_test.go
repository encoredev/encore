package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	daemonpb "encr.dev/proto/encore/daemon"
)

// TestRunCurlHTTPErrors verifies that a curl command fails when the app
// responds with an HTTP error status, unless the command sets curl's fail mode
// itself. It runs with the fail flag picked for the installed curl, and with
// the --fail fallback for curl older than 7.76.0.
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
		// errorBody is set when wantBody is an HTTP error response, which
		// curl only prints with --fail-with-body.
		errorBody bool
	}{
		{"success", "/ok", nil, true, "ok-body", false},
		{"not found", "/missing", nil, false, "missing-body", true},
		{"server error", "/fail", nil, false, "fail-body", true},
		{"user passes -f", "/missing", []string{"-f"}, false, "", false},
		{"user passes -sf", "/missing", []string{"-sf"}, false, "", false},
		{"user passes --fail", "/missing", []string{"--fail"}, false, "", false},
		{"user opts out with --no-fail", "/missing", []string{"--no-fail"}, true, "missing-body", false},
		{"other flags", "/missing", []string{"-X", "GET", "-H", "Accept: application/json"}, false, "missing-body", true},
		{"attached data value containing f", "/fail", []string{"-dfoo=bar"}, false, "fail-body", true},
		{"attached user agent containing -f", "/missing", []string{"-Afoo-f"}, false, "missing-body", true},
	}

	failFlags := []string{curlFailFlag()}
	if failFlags[0] != "--fail" {
		failFlags = append(failFlags, "--fail")
	}
	orig := curlFailFlag
	defer func() { curlFailFlag = orig }()

	for _, failFlag := range failFlags {
		curlFailFlag = func() string { return failFlag }
		for _, tt := range tests {
			t.Run(failFlag+"/"+tt.name, func(t *testing.T) {
				stdout, stderr, exitCode := runCurl(context.Background(), addr, &daemonpb.CurlCommand{Path: tt.path, Args: tt.args})
				if gotOK := exitCode == 0; gotOK != tt.wantOK {
					t.Errorf("exit code = %d, want success = %v (stderr: %q)", exitCode, tt.wantOK, stderr)
				}
				if exitCode == 2 {
					t.Errorf("curl rejected its arguments (stderr: %q)", stderr)
				}
				wantBody := tt.wantBody != "" && (!tt.errorBody || failFlag == "--fail-with-body")
				if wantBody && !strings.Contains(string(stdout), tt.wantBody) {
					t.Errorf("stdout = %q, want it to contain %q", stdout, tt.wantBody)
				}
			})
		}
	}
}

func TestSetsFailMode(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"-i"}, false},
		{[]string{"-X", "POST", "-d", `{"from":1}`}, false},
		{[]string{"-H", "x-foo: bar"}, false},
		{[]string{"-dfoo=bar"}, false},
		{[]string{"-Afoo-f"}, false},
		{[]string{"-sXPOST"}, false},
		{[]string{"-d", "-f"}, false},
		{[]string{"-H", "x: y", "-f"}, true},
		{[]string{"-sSLf"}, true},
		{[]string{"-fXPOST"}, true},
		{[]string{"-f"}, true},
		{[]string{"-sSf"}, true},
		{[]string{"--fail"}, true},
		{[]string{"--fail-with-body"}, true},
		{[]string{"--no-fail"}, true},
		{[]string{"--no-fail-with-body"}, true},
	}
	for _, tt := range tests {
		if got := setsFailMode(tt.args); got != tt.want {
			t.Errorf("setsFailMode(%q) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestParseCurlVersion(t *testing.T) {
	tests := []struct {
		out          string
		major, minor int
		ok           bool
	}{
		{"curl 8.7.1 (x86_64-apple-darwin24.0) libcurl/8.7.1 (SecureTransport)\nRelease-Date: 2024-03-27\n", 8, 7, true},
		{"curl 7.61.1 (x86_64-redhat-linux-gnu) libcurl/7.61.1\n", 7, 61, true},
		{"curl 8.9.1 (Windows) libcurl/8.9.1 Schannel\n", 8, 9, true},
		{"", 0, 0, false},
		{"not curl\n", 0, 0, false},
	}
	for _, tt := range tests {
		major, minor, ok := parseCurlVersion(tt.out)
		if major != tt.major || minor != tt.minor || ok != tt.ok {
			t.Errorf("parseCurlVersion(%q) = %d, %d, %v, want %d, %d, %v", tt.out, major, minor, ok, tt.major, tt.minor, tt.ok)
		}
	}
}
