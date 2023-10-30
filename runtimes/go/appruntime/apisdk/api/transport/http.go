package transport

import (
	"net/http"
	"sort"
	"strings"
)

// HTTPRequest returns a Transport implementation for the given HTTP request.
func HTTPRequest(req *http.Request) Transport {
	return &httpHeaders{headers: req.Header}
}

// HTTPResponse returns a Transport implementation for the given HTTP response.
func HTTPResponse(resp *http.Response) Transport {
	return &httpHeaders{headers: resp.Header}
}

// HTTPResponseWriter returns a Transport implementation for the given HTTP response.
func HTTPResponseWriter(w http.ResponseWriter) Transport {
	return &httpHeaders{headers: w.Header()}
}

// httpHeaders is a Transport implementation for HTTP requests and responses.
// which gives us a uniform way to add and read the headers from the either
// a [http.Request] or a [http.ResponseWriter].
type httpHeaders struct {
	headers http.Header
}

var _ Transport = (*httpHeaders)(nil)

func metaKeyToHTTPHeader(key string) string {
	switch key {
	case TraceParentKey:
		return "traceparent"
	case TraceStateKey:
		return "tracestate"
	case CorrelationIDKey:
		return "X-Correlation-ID"
	default:
		return "X-Encore-Meta-" + key
	}
}

func (h *httpHeaders) SetMeta(key string, value string) {
	h.headers.Set(metaKeyToHTTPHeader(key), value)
}

func (h *httpHeaders) ReadMeta(key string) (value string, found bool) {
	value = h.headers.Get(metaKeyToHTTPHeader(key))
	return value, value != ""
}

func (h *httpHeaders) ReadMetaValues(key string) (values []string, found bool) {
	values = h.headers.Values(metaKeyToHTTPHeader(key))
	return values, len(values) > 0
}

func (h *httpHeaders) ListMetaKeys() []string {
	rtn := make([]string, 0, len(h.headers))

	// List all keys
	for key := range h.headers {
		key := http.CanonicalHeaderKey(key)

		switch {
		case key == "Traceparent":
			rtn = append(rtn, TraceParentKey)
		case key == "Tracestate":
			rtn = append(rtn, TraceStateKey)
		case key == "X-Correlation-Id":
			rtn = append(rtn, CorrelationIDKey)
		case strings.HasPrefix(key, "X-Encore-Meta-"):
			rtn = append(rtn, key[14:])
		}
	}

	sort.Strings(rtn)

	return rtn
}
