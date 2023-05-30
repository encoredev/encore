package transport

import (
	"net/http"
)

// HTTPRequest returns a Transport implementation for the given HTTP request.
func HTTPRequest(req *http.Request) Transport {
	return &httpHeaders{headers: req.Header}
}

// HTTPResponse returns a Transport implementation for the given HTTP response.
func HTTPResponse(w http.ResponseWriter) Transport {
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
