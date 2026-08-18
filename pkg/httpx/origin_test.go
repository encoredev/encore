package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func withSecFetchSite(secFetchSite string) func(*http.Request) {
	return func(req *http.Request) {
		if secFetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", secFetchSite)
		}
	}
}

func withOrigin(origin string) func(*http.Request) {
	return func(req *http.Request) {
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
	}
}

func newReq(opts ...func(*http.Request)) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/x", nil)
	for _, apply := range opts {
		apply(req)
	}
	return req
}

func TestIsLocalOrigin(t *testing.T) {
	tests := []struct {
		origin string
		want   bool
	}{
		{"", true}, // non-browser client
		{"http://localhost:5173", true},
		{"http://127.0.0.1:3000", true},
		{"http://[::1]:3000", true},
		{"https://example.com", false},
		{"http://example.com:8000", false},
		{"://bad", false},
	}
	for _, tt := range tests {
		if got := IsLocalOrigin(newReq(withOrigin(tt.origin))); got != tt.want {
			t.Errorf("IsLocalOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestIsNonBrowser(t *testing.T) {
	tests := []struct {
		origin       string
		secFetchSite string
		want         bool
	}{
		{"", "", true}, // only a missing Origin + Sec-Fetch-Site are allowed
		// Origin or Sec-Fetch-Site headers are not allowd
		{"http://localhost:5173", "", false},
		{"http://localhost:5173", "", false},
		{"http://127.0.0.1:3000", "", false},
		{"https://example.com", "", false},
		{"", "same-site", false},
		{"", "same-origin", false},
		{"", "cross-site", false},
		{"", "none", false},
		{"http://localhost:3000", "same-site", false},
	}
	for _, tt := range tests {
		if got := IsNonBrowser(newReq(withOrigin(tt.origin), withSecFetchSite(tt.secFetchSite))); got != tt.want {
			t.Errorf("IsNonBrowser(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestIsNonExternalWebsite(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		fetchSite string
		want      bool
	}{
		// A cross-site GET sub-resource load (e.g. <img>) sends no Origin but
		// still carries Sec-Fetch-Site: cross-site.
		{"cross-site no origin", "", "cross-site", false},
		{"cross-site with origin", "https://example.com", "cross-site", false},
		// Same-origin/same-site requests from the local dashboard and localhost
		// dev servers must keep working.
		{"same-origin", "http://localhost:9400", "same-origin", true},
		{"same-site other port", "http://localhost:5173", "same-site", true},
		// A user navigating directly (typed URL/bookmark) reports "none".
		{"user navigation", "", "none", true},
	}
	for _, tt := range tests {
		if got := IsNotExternalWebsite(newReq(withOrigin(tt.origin), withSecFetchSite(tt.fetchSite))); got != tt.want {
			t.Errorf("%s: IsLocalOrigin(origin=%q, sec-fetch-site=%q) = %v, want %v", tt.name, tt.origin, tt.fetchSite, got, tt.want)
		}
	}
}

func TestCheckOrigin(t *testing.T) {
	handler := CheckOrigin(IsNonBrowser, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newReq())
	if rec.Code != http.StatusOK {
		t.Errorf("no-origin request: got %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, newReq(withOrigin("https://example.com")))
	if rec.Code != http.StatusForbidden {
		t.Errorf("browser request: got %d, want 403", rec.Code)
	}
}
