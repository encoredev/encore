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

	return dst
}

//go:linkname exitEncoreG runtime.exitEncoreG
func exitEncoreG(e *encoreG) {}

//go:linkname beginHTTPRoundTrip net/http.encoreBeginRoundTrip
func beginHTTPRoundTrip(req *http.Request) (context.Context, error) {
	g := getEncoreG()
	if g == nil || g.req == nil || !g.req.data.Traced {
		return req.Context(), nil
	} else if req.URL == nil {
		return nil, fmt.Errorf("http: nil Request.URL")
	}

	return g.op.trace.HTTPBeginRoundTrip(req, g.req.data, g.goctr)
}

//go:linkname finishHTTPRoundTrip net/http.encoreFinishRoundTrip
func finishHTTPRoundTrip(req *http.Request, resp *http.Response, err error) {
	if g := getEncoreG(); g != nil && g.req != nil && g.op.trace != nil {
		g.op.trace.HTTPCompleteRoundTrip(req, resp, g.goctr, err)
	}
}
