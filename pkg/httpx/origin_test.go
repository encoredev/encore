package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newReq(origin string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func newReqWithFetchSite(origin, fetchSite string) *http.Request {
	req := newReq(origin)
	if fetchSite != "" {
		req.Header.Set("Sec-Fetch-Site", fetchSite)
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
		if got := IsLocalOrigin(newReq(tt.origin)); got != tt.want {
			t.Errorf("IsLocalOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestIsLocalOriginFetchMetadata(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		fetchSite string
		want      bool
	}{
		// A cross-site GET sub-resource load (e.g. <img>) sends no Origin but
		// still carries Sec-Fetch-Site: cross-site. It must be rejected.
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
		if got := IsLocalOrigin(newReqWithFetchSite(tt.origin, tt.fetchSite)); got != tt.want {
			t.Errorf("%s: IsLocalOrigin(origin=%q, sec-fetch-site=%q) = %v, want %v", tt.name, tt.origin, tt.fetchSite, got, tt.want)
		}
	}
}

func TestIsNonBrowser(t *testing.T) {
	tests := []struct {
		origin string
		want   bool
	}{
		{"", true}, // only a missing Origin is allowed
		{"http://localhost:5173", false},
		{"http://127.0.0.1:3000", false},
		{"https://example.com", false},
	}
	for _, tt := range tests {
		if got := IsNonBrowser(newReq(tt.origin)); got != tt.want {
			t.Errorf("IsNonBrowser(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestIsNonBrowserFetchMetadata(t *testing.T) {
	tests := []struct {
		name      string
		origin    string
		fetchSite string
		want      bool
	}{
		// go tool pprof / CLIs / the app runtime send neither header.
		{"non-browser tool", "", "", true},
		// A browser GET drive-by (e.g. <img src=".../debug/pprof/profile">)
		// sends no Origin but does send Sec-Fetch-Site. It must be rejected.
		{"cross-site drive-by", "", "cross-site", false},
		{"same-origin browser", "", "same-origin", false},
		{"user navigation", "", "none", false},
	}
	for _, tt := range tests {
		if got := IsNonBrowser(newReqWithFetchSite(tt.origin, tt.fetchSite)); got != tt.want {
			t.Errorf("%s: IsNonBrowser(origin=%q, sec-fetch-site=%q) = %v, want %v", tt.name, tt.origin, tt.fetchSite, got, tt.want)
		}
	}
}

func TestCheckOrigin(t *testing.T) {
	handler := CheckOrigin(IsNonBrowser, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newReq(""))
	if rec.Code != http.StatusOK {
		t.Errorf("no-origin request: got %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, newReq("https://example.com"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("browser request: got %d, want 403", rec.Code)
	}
}
