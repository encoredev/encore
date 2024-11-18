package gcsemu

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/api/googleapi"
)

// jsonRespond json-encodes rsp and writes it to w.  If an error occurs, then it is logged and a 500 error is written to w.
func (g *GcsEmu) jsonRespond(w http.ResponseWriter, rsp interface{}) {
	// do NOT write a http status since OK will be the default and this allows the caller to use their own if they want
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(rsp); err != nil {
		g.log(err, "failed to send response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

type gapiErrorPartial struct {
	// Code is the HTTP response status code and will always be populated.
	Code int `json:"code"`

	// Message is the server response message and is only populated when
	// explicitly referenced by the JSON server response.
	Message string `json:"message"`

	Errors []googleapi.ErrorItem `json:"errors,omitempty"`
}

// gapiError responds to the client with a GAPI error
func (g *GcsEmu) gapiError(w http.ResponseWriter, code int, message string) {
	if code == 0 {
		code = http.StatusInternalServerError
	}
	if code != http.StatusNotFound {
		g.log(errors.New(message), "responding with HTTP %d", code)
	}
	if message == "" {
		message = http.StatusText(code)
	}

	// format copied from errorReply struct in google.golang.org/api/googleapi
	rsp := struct {
		Error gapiErrorPartial `json:"error"`
	}{
		Error: gapiErrorPartial{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(&rsp)
}

// mustJson serializes the given value to json, panicking on failure
func mustJson(val interface{}) []byte {
	if val == nil {
		return []byte("null")
	}

	b, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

// requestHost returns the host from an http.Request, respecting proxy headers. Works locally with devproxy
// and gulp proxies as well as in AppEngine (both real GAE and the dev_appserver).
func requestHost(req *http.Request) string {
	// proxies like gulp are supposed to accumulate original host, next-step-host, etc in order from
	// client-most to server-most in X-ForwardedHost; return the first entry from that if any are listed
	if proxyHost := req.Header.Get("X-Forwarded-Host"); proxyHost != "" {
		// Use the first (closest to client) host
		splits := strings.SplitN(proxyHost, ",", 2)
		return splits[0]
	}

	// Forwarded is the standardized version of X-Forwarded-Host.
	f := parseForwardedHeader(req.Header.Get("Forwarded"))
	if len(f.Host) > 0 && len(f.Host[0]) > 0 {
		return f.Host[0]
	}

	// Clients that generate HTTP/2 requests should use the :authority header instead
	// of Host. See http://httpwg.org/specs/rfc7540.html#rfc.section.8.1.2.3
	host := req.Header.Get("Authority")
	if len(host) > 0 {
		return host
	}

	// Fall back to the host line.
	return req.Host
}

// forwarded represents the values of a Forwarded HTTP header.
//
// For more details, see the RFC: https://tools.ietf.org/html/rfc7239 and
// MDN: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded.
type forwarded struct {
	By    []string
	For   []string
	Host  []string
	Proto []string
}

var (
	forwardedHostRx = regexp.MustCompile(`(?i)host=(.*?)(?:[,;\s]|$)`)
)

func removeDoubleQuotes(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, `"`), `"`)
}

// Note: this currently only supports the forwarded.Host field.
func parseForwardedHeader(s string) forwarded {
	var f forwarded

	if s == "" {
		return f
	}

	matches := forwardedHostRx.FindAllStringSubmatch(s, -1)
	for _, m := range matches {
		if len(m) > 0 {
			f.Host = append(f.Host, removeDoubleQuotes(m[1]))
		}
	}

	return f
}
