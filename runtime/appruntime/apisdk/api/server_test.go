package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/julienschmidt/httprouter"
)

func Test_handleTrailingSlashRedirect(t *testing.T) {
	r := httprouter.New()
	dummy := func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {}
	r.GET("/foo", dummy)
	r.GET("/bar/", dummy)
	r.POST("/post", dummy)

	tests := []struct {
		// inputs
		method, path string
		// outputs
		handled bool
		code    int
		dest    string
	}{
		// Matches existing routes
		{"GET", "/foo", false, 0, ""},
		{"GET", "/bar/", false, 0, ""},

		// Redirect to with (without) trailing slash
		{"GET", "/foo/", true, http.StatusMovedPermanently, "/foo"},
		{"GET", "/bar", true, http.StatusMovedPermanently, "/bar/"},
		{"POST", "/post/", true, http.StatusPermanentRedirect, "/post"},

		// Unknown routes
		{"GET", "/baz", false, 0, ""},
		{"GET", "/baz/", false, 0, ""},
	}

	for _, test := range tests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(test.method, test.path, nil)
		handled := handleTrailingSlashRedirect(r, w, req, test.path)
		if !handled && !test.handled {
			continue
		} else if handled != test.handled {
			t.Errorf("%s %s: got handled=%v, want %v", test.method, test.path, handled, test.handled)
			continue
		}

		if w.Code != test.code {
			t.Errorf("%s %s: got code=%d, want %d", test.method, test.path, w.Code, test.code)
		} else if w.Header().Get("Location") != test.dest {
			t.Errorf("%s %s: got dest=%s, want %s", test.method, test.path, w.Header().Get("Location"), test.dest)
		}
	}
}

func Test_determineRequestPath(t *testing.T) {
	tests := []struct {
		path    string
		rawPath string
		want    string
	}{
		{
			path:    "/{foo}",
			rawPath: "",
			want:    "/{foo}",
		},
		{
			path:    "/foo/bar/baz",
			rawPath: "/foo/bar%2Fbaz",
			want:    "/foo/bar%2Fbaz",
		},
	}
	for _, tt := range tests {
		u := &url.URL{Path: tt.path, RawPath: tt.rawPath}
		if got := determineRequestPath(u); got != tt.want {
			t.Errorf("determineRequestPath(&url.URL{Path: %q, RawPath: %q}) = %q, want %q",
				tt.path, tt.rawPath, got, tt.want)
		}
	}
}
