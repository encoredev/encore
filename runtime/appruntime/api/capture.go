package api

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/felixge/httpsnoop"
)

// rawRequestBodyCapturer is an io.ReadCloser that can be substituted for
// (*http.Request).Body to capture its payload for tracing raw requests.
//
// It only parses requests matching certain mime types and only
// a limited amount of data suitable for displaying in traces.
type rawRequestBodyCapturer struct {
	state      captureState
	underlying io.ReadCloser

	bufMu       sync.Mutex
	buf         *bytes.Buffer
	overflowed  bool        // whether the buffer overflowed
	httpMethod  string      // The HTTP method of the request
	headers     http.Header // The headers of the request
	queryString url.Values  // The query string of the request captured
}

func newRawRequestBodyCapturer(req *http.Request) *rawRequestBodyCapturer {
	buf := captureBufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	var state captureState
	// Don't capture WebSockets etc
	if isUpgradeRequest(req) {
		state = notCapturing
	} else {
		state = shouldCaptureContentType(req.Header.Get("Content-Type"), false)
	}

	var queryString url.Values
	if req.URL != nil {
		queryString = req.URL.Query()
	}

	return &rawRequestBodyCapturer{
		state:       state,
		underlying:  req.Body,
		buf:         buf,
		httpMethod:  req.Method,
		headers:     req.Header,
		queryString: queryString,
	}
}

// captureState defines whether the capturer is actively capturing.
type captureState uint8

const (
	notCapturing captureState = 0
	capturing    captureState = 1

	// peeking means the capturer is evaluating whether or not to capture
	// the full request, based on peeking at the first few bytes of
	// the request. It's used when the Content-Type is missing or vague.
	peeking captureState = 2
)

const (
	// MaxRawRequestCaptureLen is the maximum buffer size to keep for
	// captured request payloads.
	MaxRawRequestCaptureLen  = 10 << 10  // 10 KiB
	MaxRawResponseCaptureLen = 100 << 10 // 100 KiB
)

// Read implements io.Reader.
func (c *rawRequestBodyCapturer) Read(b []byte) (int, error) {
	n, err := c.underlying.Read(b)

	if n > 0 && c.state != notCapturing && !c.overflowed {
		c.bufMu.Lock()
		defer c.bufMu.Unlock()

		// Guard against the buffer having been disposed of.
		if c.buf != nil {
			remaining := MaxRawRequestCaptureLen - c.buf.Len()
			toWrite := n
			// If we can write less than we read, mark the buffer as having overflowed.
			if remaining < n {
				toWrite = remaining
				c.overflowed = true
			}
			c.buf.Write(b[:toWrite])

			if c.state == peeking && c.buf.Len() >= 512 {
				contentType := http.DetectContentType(c.buf.Bytes())
				c.state = shouldCaptureContentType(contentType, true)
			}
		}
	}

	return n, err
}

// Close implements io.Closer.
func (c *rawRequestBodyCapturer) Close() error {
	return c.underlying.Close()
}

// FinishCapturing finishes the capturing, returning the captured bytes thus far.
// The returned buf may be used until Dispose is called but not after that.
//
// If c is nil it reports nil, false.
func (c *rawRequestBodyCapturer) FinishCapturing() (method string, contentType string, data []byte, overflowed bool) {
	if c == nil {
		return "", "", nil, false
	}

	c.bufMu.Lock()
	defer c.bufMu.Unlock()

	// If we're definitely not capturing, return no data.
	if c.state == notCapturing {
		return "", "", nil, false
	}

	// If we're peeking, make a decision.
	data = c.buf.Bytes()
	if c.state == peeking {
		contentType := http.DetectContentType(data)
		c.state = shouldCaptureContentType(contentType, true)
		if c.state != capturing {
			// Not capturing after all; return no data.
			return "", "", nil, false
		}
	}

	// Stop capturing.
	c.state = notCapturing
	return c.httpMethod, c.headers.Get("Content-Type"), data, c.overflowed
}

// Dispose disposes of the capturer, returning the
// resources to the pool for use by future requests.
//
// The capturer may not be used after calling Dispose.
func (c *rawRequestBodyCapturer) Dispose() {
	c.bufMu.Lock()
	defer c.bufMu.Unlock()
	if c.buf != nil {
		captureBufferPool.Put(c.buf)
		c.buf = nil
	}
}

var captureBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func shouldCaptureContentType(contentType string, didPeek bool) captureState {
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = contentType[:idx]
	}

	switch contentType {
	case "application/json", "text/plain", "application/x-www-form-urlencoded", "text/csv",
		"text/javascript", "application/ld+json", "application/xml", "text/xml", "application/atom+xml",
		"application/graphql":
		return capturing

	// Unknown content type; peek at the data to decide what to do.
	case "", "application/octet-stream":
		// If we already peeked, don't capture these.
		if didPeek {
			return notCapturing
		}
		return peeking

	default:
		return notCapturing
	}
}

func isUpgradeRequest(req *http.Request) bool {
	return req.Header.Get("Upgrade") != ""
}

type rawResponseCapturer struct {
	state captureState
	w     http.ResponseWriter
	hooks httpsnoop.Hooks

	Code          int
	HeaderWritten bool
	BytesWritten  int64

	// Header is a snapshot of the HTTP headers when they get written.
	Header http.Header

	bufMu      sync.Mutex
	buf        *bytes.Buffer
	overflowed bool // whether the buffer overflowed
}

func newRawResponseCapturer(w http.ResponseWriter, req *http.Request) *rawResponseCapturer {
	buf := captureBufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	var state captureState
	// Don't capture WebSockets etc
	if isUpgradeRequest(req) {
		state = notCapturing
	} else {
		state = peeking
	}

	c := &rawResponseCapturer{
		state: state,
		w:     w,
		buf:   buf,
	}
	c.hooks = httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(code int) {
				c.onWriteHeader(code, next)
			}
		},

		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(p []byte) (int, error) {
				return c.onWrite(p, next)
			}
		},

		ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
			return func(src io.Reader) (int64, error) {
				return c.onReadFrom(src, next)
			}
		},
	}
	return c
}

// InvokeHandler invokes the given HTTP handler, using the response capturer to
// capture information about the response.
func (c *rawResponseCapturer) InvokeHandler(h http.Handler, req *http.Request) {
	c.Code = 200 // by default
	w := httpsnoop.Wrap(c.w, c.hooks)
	h.ServeHTTP(w, req)
}

func (c *rawResponseCapturer) onWriteHeader(code int, next httpsnoop.WriteHeaderFunc) {
	next(code)
	c.markHeadersWritten(code)
}

func (c *rawResponseCapturer) onWrite(p []byte, next httpsnoop.WriteFunc) (int, error) {
	n, err := next(p)
	c.markHeadersWritten(200)
	c.BytesWritten += int64(n)
	c.writeToBuf(p[:n])
	return n, err
}

func (c *rawResponseCapturer) onReadFrom(src io.Reader, next httpsnoop.ReadFromFunc) (int64, error) {
	if c.state != notCapturing {
		// If we're capturing data,
		src = io.TeeReader(src, rawResponseCapturerTeeWriter{c})
	}
	n, err := next(src)
	c.markHeadersWritten(200)
	c.BytesWritten += n
	return n, err
}

// writeToBuf writes data to the capture buffer, if we're capturing data.
func (c *rawResponseCapturer) writeToBuf(b []byte) {
	n := len(b)

	if n > 0 && c.state != notCapturing && !c.overflowed {
		c.bufMu.Lock()
		defer c.bufMu.Unlock()

		// Guard against the buffer having been disposed of.
		if c.buf != nil {
			remaining := MaxRawResponseCaptureLen - c.buf.Len()
			toWrite := n
			// If we can write less than we read, mark the buffer as having overflowed.
			if remaining < n {
				toWrite = remaining
				c.overflowed = true
			}
			c.buf.Write(b[:toWrite])

			if c.state == peeking && c.buf.Len() >= 512 {
				contentType := http.DetectContentType(c.buf.Bytes())
				c.state = shouldCaptureContentType(contentType, true)
			}
		}
	}
}

// markHeadersWritten marks the headers as written, using the given status code
// and snapshotting the HTTP headers written.
// If the headers have already been written, this is a no-op.
func (c *rawResponseCapturer) markHeadersWritten(code int) {
	if c.HeaderWritten {
		return
	}
	c.Code = code
	c.HeaderWritten = true

	// Snapshot the headers
	src := c.w.Header()
	c.Header = make(http.Header, len(src))
	for k, v := range src {
		c.Header[k] = v
	}
}

// FinishCapturing finishes the capturing, returning the captured bytes thus far.
// The returned buf may be used until Dispose is called but not after that.
//
// If c is nil it reports nil, false.
func (c *rawResponseCapturer) FinishCapturing() (data []byte, overflowed bool) {
	if c == nil {
		return nil, false
	}

	c.bufMu.Lock()
	defer c.bufMu.Unlock()

	// If we're definitely not capturing, return no data.
	if c.state == notCapturing {
		return nil, false
	}

	// If we're peeking, make a decision.
	data = c.buf.Bytes()
	if c.state == peeking {
		contentType := http.DetectContentType(data)
		c.state = shouldCaptureContentType(contentType, true)
		if c.state != capturing {
			// Not capturing after all; return no data.
			return nil, false
		}
	}

	// Stop capturing.
	c.state = notCapturing
	return data, c.overflowed
}

// Dispose disposes of the capturer, returning the
// resources to the pool for use by future requests.
//
// The capturer may not be used after calling Dispose.
func (c *rawResponseCapturer) Dispose() {
	c.bufMu.Lock()
	defer c.bufMu.Unlock()
	if c.buf != nil {
		captureBufferPool.Put(c.buf)
		c.buf = nil
	}
}

type rawResponseCapturerTeeWriter struct {
	c *rawResponseCapturer
}

// Write implements io.Writer, for the use with onReadFrom.
func (w rawResponseCapturerTeeWriter) Write(p []byte) (int, error) {
	// onReadFrom is handling the updating of status code, headers, and bytes written,
	// so we only write to our buffer here.
	w.c.writeToBuf(p)
	return len(p), nil
}
