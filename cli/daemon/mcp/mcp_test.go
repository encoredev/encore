package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"encr.dev/pkg/httpx"
)

// TestOriginCheck verifies the MCP listener only serves code agents (no Origin
// header) and rejects any browser request (which always carries an Origin).
func TestOriginCheck(t *testing.T) {
	handler := httpx.CheckOrigin(httpx.IsNonBrowser, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{"no origin (code agent)", "", true},
		{"localhost origin", "http://localhost:5173", false},
		{"loopback ip origin", "http://127.0.0.1:3000", false},
		{"remote origin", "https://example.com", false},
		{"remote origin with port", "http://example.com:8000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9900/sse", nil)
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
