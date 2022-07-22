package api

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/felixge/httpsnoop"
	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"

	"encore.dev/appruntime/model"
	"encore.dev/beta/errs"
)

type PathParams = httprouter.Params

type Void struct{}

func (*Void) Serialize(json jsoniter.API) ([][]byte, error) {
	return [][]byte{[]byte("{}")}, nil
}

func (*Void) Clone() (*Void, error) {
	return &Void{}, nil
}

type Serializable interface {
	Serialize(json jsoniter.API) ([][]byte, error)
}

type Clonable[Self any] interface {
	Serializable
	Clone() (Self, error)
}

type RequestSpec[Self any] interface {
	Clonable[Self]
	Path() (path string, params PathParams, err error)
}

// Desc is a description of an API handler.
type Desc[Req RequestSpec[Req], Resp Clonable[Resp]] struct {
	// Service and Endpoint name the API this description is for.
	Service  string
	Endpoint string
	Methods  []string
	Path     string
	DefLoc   int32

	// Access describes the access type for this API.
	Access Access

	// If raw is true, RawHandler is set and AppHandler and EncodeResp are nil.
	Raw bool

	DecodeReq  func(*http.Request, PathParams, jsoniter.API) (Req, error)
	AppHandler func(context.Context, Req) (Resp, error)
	EncodeResp func(http.ResponseWriter, jsoniter.API, Resp) error
	RawHandler func(http.ResponseWriter, *http.Request)
}

func (d *Desc[Req, Resp]) AccessType() Access    { return d.Access }
func (d *Desc[Req, Resp]) ServiceName() string   { return d.Service }
func (d *Desc[Req, Resp]) EndpointName() string  { return d.Endpoint }
func (d *Desc[Req, Resp]) HTTPMethods() []string { return d.Methods }
func (d *Desc[Req, Resp]) HTTPPath() string      { return d.Path }

func (d *Desc[Req, Resp]) Handle(c Context) {
	reqData, err := d.begin(c)
	if err != nil {
		errs.HTTPError(c.w, err)
		return
	}

	resp, httpStatus, err := d.executeEndpoint(c, reqData)
	if err != nil {
		c.server.finishRequest(nil, err, httpStatus)
		errs.HTTPError(c.w, err)
		return
	}

	var output [][]byte
	if !d.Raw {
		err = d.EncodeResp(c.w, c.server.json, resp)
		output, _ = resp.Serialize(c.server.json)
	}
	c.server.finishRequest(output, err, httpStatus)
}

func (d *Desc[Req, Resp]) begin(c Context) (reqData Req, beginErr error) {
	reqData, decodeErr := d.DecodeReq(c.req, c.ps, c.server.json)

	if d.Access == RequiresAuth && c.auth.UID == "" {
		beginErr = errs.B().
			Code(errs.Unauthenticated).
			Meta("service", d.Service, "endpoint", d.Endpoint).
			Msg("endpoint requires auth but none provided").
			Err()
		return
	}

	inputs, _ := reqData.Serialize(c.server.json)
	err := c.server.beginRequest(c.req.Context(), &beginRequestParams{
		Type:     model.RPCCall,
		Service:  d.Service,
		Endpoint: d.Endpoint,
		DefLoc:   d.DefLoc,

		Path:         c.req.URL.Path,
		PathSegments: c.ps,
		Inputs:       inputs,

		UID:      c.auth.UID,
		AuthData: c.auth.UserData,
	})
	if err != nil {
		beginErr = errs.B().Code(errs.Internal).Msg("internal error").Err()
		return
	}

	// If we fail after having begun the request, mark it as completed.
	defer func() {
		if beginErr != nil {
			c.server.finishRequest(nil, beginErr, 0)
		}
	}()

	if decodeErr != nil {
		beginErr = decodeErr
		return
	}

	return reqData, nil
}

func (d *Desc[Req, Resp]) executeEndpoint(c Context, reqData Req) (resp Resp, httpStatus int, respErr error) {
	defer func() {
		// Catch endpoint panic
		if e := recover(); e != nil {
			stack := debug.Stack()
			respErr = errs.B().Code(errs.Internal).Meta("panic_stack", string(stack)).Msgf("panic handling request: %v\n%s", e, stack).Err()
		}
	}()

	if d.Raw {
		m := httpsnoop.CaptureMetrics(http.HandlerFunc(d.RawHandler), c.w, c.req)
		httpStatus = m.Code
		if m.Code >= 400 {
			err := fmt.Errorf("response status code %d", m.Code)
			respErr = errs.Convert(err)
		}
		return
	}

	resp, err := d.AppHandler(c.req.Context(), reqData)
	if err != nil {
		respErr = errs.Convert(err)
		return
	}

	return resp, 0, nil
}

type CallContext struct {
	ctx    context.Context
	server *Server
}

func (d *Desc[Req, Resp]) Call(c CallContext, req Req) (resp Resp, respErr error) {
	inputs, err := req.Serialize(c.server.json)
	if err != nil {
		respErr = err
		return
	}

	path, params, err := req.Path()
	if err != nil {
		respErr = err
		return
	}

	call, err := c.server.beginCall()
	if respErr != nil {
		respErr = err
		return
	}

	// Run the request in a different goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		beginErr := c.server.beginRequest(c.ctx, &beginRequestParams{
			Type:     model.RPCCall,
			Service:  d.Service,
			Endpoint: d.Endpoint,
			DefLoc:   d.DefLoc,

			Path:         path,
			PathSegments: params,
			Inputs:       inputs,
			SpanID:       call.SpanID,
		})
		if beginErr != nil {
			respErr = errs.B().Code(errs.Internal).Msg("internal error").Err()
			return
		}

		// Handle panics in the request handler
		defer func() {
			if err := recover(); err != nil {
				respErr = errs.B().Code(errs.Internal).Msgf("panic handling request: %v", err).Err()
				c.server.finishRequest(nil, respErr, 0)
			}
		}()

		r, rpcErr := d.AppHandler(c.ctx, req)
		if rpcErr == nil {
			r, rpcErr = r.Clone()
		}
		if rpcErr != nil {
			respErr = errs.RoundTrip(rpcErr)
			c.server.finishRequest(nil, respErr, 0)
		} else {
			resp, respErr = r, nil
			output, _ := r.Serialize(c.server.json)
			c.server.finishRequest(output, nil, 0)
		}
	}()
	<-done

	c.server.finishCall(call, respErr)
	return
}
