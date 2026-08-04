package objects

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"encr.dev/pkg/httpx"
)

// TestOriginCheck verifies the public bucket server serves non-browser clients
// (no Origin header) and local browser origins, but rejects cross-site origins
// so a malicious website can't drive-by request the localhost-bound server.
func TestOriginCheck(t *testing.T) {
	handler := httpx.CheckOrigin(httpx.IsLocalOrigin, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{"no origin (runtime/cli)", "", true},
		{"localhost origin", "http://localhost:5173", true},
		{"loopback ip origin", "http://127.0.0.1:3000", true},
		{"loopback ipv6 origin", "http://[::1]:3000", true},
		{"remote origin", "https://example.com", false},
		{"remote origin with port", "http://example.com:8000", false},
		{"malformed origin", "://bad", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9800/ns/b/obj", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			allowed := rec.Code == http.StatusOK
			if allowed != tt.allowed {
				t.Fatalf("origin=%q: got status %d (allowed=%v), want allowed=%v",
					tt.origin, rec.Code, allowed, tt.allowed)
			}
		})
	}
}
