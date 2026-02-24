//go:build encore_app

package reqtrack

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	_ "unsafe" // for go:linkname
)

func newImpl() reqTrackImpl {
	return appImpl{}
}

type appImpl struct{}

var _ reqTrackImpl = appImpl{}

func (appImpl) get() *encoreG {
	return getEncoreG()
}

func (appImpl) set(val *encoreG) {
	setEncoreG(val)
}

// getEncoreG gets the encore data for the current g, or nil.
//
//go:linkname getEncoreG runtime.getEncoreG
func getEncoreG() *encoreG

// setEncoreG sets the encore data for the current g to val.
//
//go:linkname setEncoreG runtime.setEncoreG
func setEncoreG(val *encoreG)

//go:linkname startEncoreG runtime.startEncoreG
func startEncoreG(src *encoreG) *encoreG {
	if src == nil {
		return nil
	}

	goctr := atomic.AddUint32(&src.op.goidCtr, 1)
	dst := &encoreG{
		op:    src.op,
		req:   src.req,
		goctr: goctr,
	}

	// Copy the span stack so the child goroutine inherits the active custom span.
	if len(src.spanStack) > 0 {
		dst.spanStack = make([]activeSpan, len(src.spanStack))
		copy(dst.spanStack, src.spanStack)
	}

	return dst
}

//go:linkname exitEncoreG runtime.exitEncoreG
func exitEncoreG(e *encoreG) {}

// ActiveSpanIDKey is the context key used to pass the active span ID
// to the HTTP round trip tracer. This allows the tracer to emit events
// under the correct span (which may be a custom span).
var ActiveSpanIDKey = activeSpanIDKeyType{}

type activeSpanIDKeyType struct{}

//go:linkname beginHTTPRoundTrip net/http.encoreBeginRoundTrip
func beginHTTPRoundTrip(req *http.Request) (context.Context, error) {
	g := getEncoreG()
	if g == nil || g.req == nil || !g.req.data.Traced {
		return req.Context(), nil
	} else if req.URL == nil {
		return nil, fmt.Errorf("http: nil Request.URL")
	}

	if trace := g.op.trace.Load(); trace != nil {
		// Resolve the active span ID (custom span if active, otherwise request span).
		activeSpanID := g.req.data.SpanID
		if len(g.spanStack) > 0 {
			activeSpanID = g.spanStack[len(g.spanStack)-1].spanID
		}
		ctx := context.WithValue(req.Context(), ActiveSpanIDKey, activeSpanID)
		req = req.WithContext(ctx)
		return trace.Logger().HTTPBeginRoundTrip(req, g.req.data, g.goctr)
	}

	return req.Context(), nil
}

//go:linkname finishHTTPRoundTrip net/http.encoreFinishRoundTrip
func finishHTTPRoundTrip(req *http.Request, resp *http.Response, err error) {
	if g := getEncoreG(); g != nil && g.req != nil {
		if trace := g.op.trace.Load(); trace != nil {
			trace.Logger().HTTPCompleteRoundTrip(req, resp, g.goctr, err)
		}
	}
}
