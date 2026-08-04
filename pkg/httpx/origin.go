// Package httpx provides small HTTP helpers shared across the daemon.
package httpx

import (
	"net"
	"net/http"
	"net/url"
)

// IsLocalOrigin gates localhost-bound dev servers so a malicious website can't
// drive-by request them, while still serving the local dashboard and frontend
// dev servers (which run on localhost).
//
// Browser requests are allowed only when the Origin's host is loopback or
// "localhost"; other origins are rejected. Requests without an Origin are
// allowed, since non-browser clients (the app runtime, CLI tools) don't send
// one. Browsers also omit Origin on GET sub-resource loads (<img>, <script>),
// so we additionally reject Sec-Fetch-Site: cross-site to catch those.
func IsLocalOrigin(req *http.Request) bool {
	if req.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}

	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	hostname := u.Hostname()
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.IsLoopback()
	}
	return hostname == "localhost"
}

// IsNonBrowser reports whether the request comes from a non-browser client. Use
// it for servers with no legitimate browser clients at all (e.g. pprof, the
// runtime trace ingest).
//
// Browsers send an Origin on cross-origin fetch/POST, and Sec-Fetch-Site on
// every request — including the Origin-less GET sub-resource loads (<img>,
// <script>) that an Origin-only check would miss. Non-browser clients send
// neither.
func IsNonBrowser(req *http.Request) bool {
	return req.Header.Get("Origin") == "" && req.Header.Get("Sec-Fetch-Site") == ""
}

// CheckOrigin returns middleware that serves a request only when allow reports
// true for it, and otherwise responds 403 Forbidden. Pair it with IsLocalOrigin
// or IsNonBrowser depending on whether the server has legitimate local browser
// clients.
func CheckOrigin(allow func(*http.Request) bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allow(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
