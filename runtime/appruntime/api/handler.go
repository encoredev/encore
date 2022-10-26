package api

import (
	"context"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"sync"

	"github.com/felixge/httpsnoop"
	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"

	encore "encore.dev"
	"encore.dev/appruntime/model"
	"encore.dev/beta/errs"
	"encore.dev/middleware"
)

type PathParams = httprouter.Params

type Void struct{}

// SerializeVoid serializes the Void type. It's called by generated code.
func SerializeVoid(json jsoniter.API, _ Void) ([][]byte, error) {
	return [][]byte{[]byte("{}")}, nil
}

// CloneVoid clones the Void type. It's called by generated code.
func CloneVoid(Void) (Void, error) {
	return Void{}, nil
}

// isVoid reports whether a generic type is Void.
func isVoid[T any]() bool {
	var zero T
	_, ok := any(zero).(Void)
	return ok
}

// Desc is a description of an API handler.
type Desc[Req, Resp any] struct {
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

	DecodeReq      func(*http.Request, PathParams, jsoniter.API) (Req, error)
	CloneReq       func(Req) (Req, error)
	SerializeReq   func(jsoniter.API, Req) ([][]byte, error)
	ReqPath        func(Req) (path string, params PathParams, err error)
	ReqUserPayload func(Req) any

	AppHandler func(context.Context, Req) (Resp, error)
	RawHandler func(http.ResponseWriter, *http.Request)

	EncodeResp    func(http.ResponseWriter, jsoniter.API, Resp) error
	SerializeResp func(jsoniter.API, Resp) ([][]byte, error)
	CloneResp     func(Resp) (Resp, error)

	// middleware is the ordered list of middleware to invoke before
	// calling the API handler. It's set with SetMiddleware during setup.
	middleware []*Middleware

	rpcDescOnce   sync.Once
	cachedRPCDesc *model.RPCDesc
}

func (d *Desc[Req, Resp]) AccessType() Access            { return d.Access }
func (d *Desc[Req, Resp]) ServiceName() string           { return d.Service }
func (d *Desc[Req, Resp]) EndpointName() string          { return d.Endpoint }
func (d *Desc[Req, Resp]) HTTPMethods() []string         { return d.Methods }
func (d *Desc[Req, Resp]) HTTPPath() string              { return d.Path }
func (d *Desc[Req, Resp]) SetMiddleware(m []*Middleware) { d.middleware = m }

func (d *Desc[Req, Resp]) Handle(c IncomingContext) {
	reqData, err := d.begin(c)
	if err != nil {
		errs.HTTPError(c.w, err)
		return
	}

	resp, httpStatus, err := d.handleIncoming(c, reqData)
	if err != nil {
		c.server.finishRequest(nil, err, httpStatus)

		// If the endpoint is raw it has already written its response;
		// don't write another.
		if !d.Raw {
			errs.HTTPErrorWithCode(c.w, err, httpStatus)
		}

		return
	}

	var output [][]byte
	if !d.Raw {
		err = d.EncodeResp(c.w, c.server.json, resp)
		output, _ = d.SerializeResp(c.server.json, resp)
	}
	c.server.finishRequest(output, err, httpStatus)
}

func (d *Desc[Req, Resp]) begin(c IncomingContext) (reqData Req, beginErr error) {
	reqData, decodeErr := d.DecodeReq(c.req, c.ps, c.server.json)

	if d.Access == RequiresAuth && c.auth.UID == "" {
		beginErr = errs.B().
			Code(errs.Unauthenticated).
			Meta("service", d.Service, "endpoint", d.Endpoint).
			Msg("endpoint requires auth but none provided").
			Err()
		return
	}

	// Only compute inputs and payload if we have valid reqData.
	var (
		inputs  [][]byte
		payload any
	)
	if decodeErr == nil {
		inputs, _ = d.SerializeReq(c.server.json, reqData)
		payload = d.ReqUserPayload(reqData)
	}

	_, err := c.server.beginRequest(c.ctx, &beginRequestParams{
		Type:     model.RPCCall,
		Service:  d.Service,
		Endpoint: d.Endpoint,
		DefLoc:   d.DefLoc,

		Path:         c.req.URL.Path,
		PathSegments: c.ps,
		Payload:      payload,
		Inputs:       inputs,
		RPCDesc:      d.rpcDesc(),

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
		beginErr = errs.WrapCode(decodeErr, errs.InvalidArgument, "decode request")
		return
	}

	return reqData, nil
}

// handleIncoming executes the given handler, running middleware in the process.
func (d *Desc[Req, Resp]) handleIncoming(c IncomingContext, reqData Req) (resp Resp, httpStatus int, respErr error) {
	if err := d.validate(reqData); err != nil {
		respErr = err
		httpStatus = errs.HTTPStatus(err)
		return
	}

	invokeHandler := func(mwReq middleware.Request) (mwResp middleware.Response) {
		if d.Raw {
			return d.invokeHandlerRaw(mwReq, c)
		} else {
			return d.invokeHandlerNonRaw(mwReq, reqData)
		}
	}
	return d.executeEndpoint(c.execContext, invokeHandler)
}

// invokeHandlerNonRaw invokes the handler for a regular (non-raw) endpoint. If the endpoint is raw, it panics.
func (d *Desc[Req, Resp]) invokeHandlerNonRaw(mwReq middleware.Request, reqData Req) (mwResp middleware.Response) {
	if d.Raw {
		panic("invokeHandlerNonRaw called on Raw endpoint")
	}
	handlerResp, handlerErr := d.AppHandler(mwReq.Context(), reqData)
	if handlerErr != nil {
		mwResp.Err = errs.Convert(handlerErr)
		mwResp.HTTPStatus = errs.HTTPStatus(mwResp.Err)
	} else {
		// Only assign the payload if we're not dealing with *Void,
		// otherwise we would end up making Payload a "typed nil".
		if !isVoid[Resp]() {
			mwResp.Payload = handlerResp
		}
		mwResp.HTTPStatus = 200
	}
	return mwResp
}

// invokeHandlerRaw invokes the handler for a raw endpoint. If the endpoint is not raw, it panics.
func (d *Desc[Req, Resp]) invokeHandlerRaw(mwReq middleware.Request, c IncomingContext) (mwResp middleware.Response) {
	if !d.Raw {
		panic("invokeHandlerRaw called on non-Raw endpoint")
	}

	// Middleware can override the context, so check if the context is different
	// and if so change the request context.
	httpReq := c.req
	if ctx := mwReq.Context(); ctx != c.req.Context() {
		httpReq = httpReq.WithContext(ctx)
	}
	m := httpsnoop.CaptureMetrics(http.HandlerFunc(d.RawHandler), c.w, httpReq)

	if m.Code >= 400 {
		statusText := http.StatusText(m.Code)
		if statusText == "" {
			statusText = "HTTP " + strconv.Itoa(m.Code)
		}
		mwResp.Err = errs.B().Code(errs.HTTPStatusToCode(m.Code)).Msg(statusText).Err()
	}
	mwResp.HTTPStatus = m.Code
	return mwResp
}

// executeEndpoint executes the given handler, running middleware in the process.
func (d *Desc[Req, Resp]) executeEndpoint(c execContext, invokeHandler func(middleware.Request) middleware.Response) (resp Resp, httpStatus int, respErr error) {
	var counter int
	var nextFn middleware.Next
	numMiddleware := len(d.middleware)
	nextFn = func(req middleware.Request) (resp middleware.Response) {
		idx := counter
		counter++

		switch {
		case idx < numMiddleware:
			mw := d.middleware[idx]
			defer func() {
				// Catch middleware panic
				if e := recover(); e != nil {
					stack := debug.Stack()
					resp.Err = errs.B().Code(errs.Internal).Meta("panic_stack", string(stack)).Msgf("panic executing middleware %s.%s: %v\n%s",
						mw.PkgName, mw.Name, e, stack).Err()
					resp.HTTPStatus = 500
				}
			}()
			return mw.Invoke(req, nextFn)

		case idx == numMiddleware:
			defer func() {
				// Catch handler panic
				if e := recover(); e != nil {
					stack := debug.Stack()
					resp.Err = errs.B().Code(errs.Internal).Meta("panic_stack", string(stack)).Msgf("panic handling request: %v\n%s", e, stack).Err()
					resp.HTTPStatus = 500
				}
			}()
			return invokeHandler(req)

		default:
			return middleware.Response{
				Err:        errs.B().Code(errs.Internal).Msg("middleware called next() too many times").Err(),
				HTTPStatus: 500,
			}
		}
	}

	// Only create the middleware.Request object if we actually have middleware.
	mwReq := middleware.NewLazyRequest(c.ctx, func() *encore.Request {
		return c.server.encoreMgr.CurrentRequest()
	})
	mwResp := nextFn(mwReq)

	if mwResp.Err != nil {
		httpStatus := mwResp.HTTPStatus
		if httpStatus == 0 {
			// If no explicit HTTP status has been set, then we use the default for the type of error
			httpStatus = errs.HTTPStatus(mwResp.Err)
		}
		return resp, httpStatus, mwResp.Err
	} else {
		httpStatus := mwResp.HTTPStatus
		if httpStatus == 0 {
			// If no explicit HTTP status has been set, then we use 200 OK
			httpStatus = 200
		}

		if resp, ok := mwResp.Payload.(Resp); ok || isVoid[Resp]() {
			return resp, httpStatus, nil
		}

		return resp, 500, errs.B().Code(errs.Internal).Msgf(
			"invalid middleware: cannot return payload of type %T for endpoint %s.%s (expected type %T)",
			mwResp.Payload, d.Service, d.Endpoint, resp,
		).Err()
	}
}

type CallContext struct {
	ctx    context.Context
	server *Server
}

func (d *Desc[Req, Resp]) Call(c CallContext, req Req) (resp Resp, respErr error) {
	// TODO: we don't currently support service-to-service calls of raw endpoints.
	// To fix this we need to improve our request serialization and DI support to
	// separate the signature for outgoing calls versus handlers.
	if d.Raw {
		respErr = errs.B().Code(errs.Internal).Msg("internal encore error: cannot call raw endpoints in service-to-service calls").Err()
		return
	}

	req, err := d.CloneReq(req)
	if err != nil {
		respErr = errs.Convert(err)
		return
	}

	inputs, err := d.SerializeReq(c.server.json, req)
	if err != nil {
		respErr = errs.Convert(err)
		return
	}

	path, params, err := d.ReqPath(req)
	if err != nil {
		respErr = errs.Convert(err)
		return
	}

	call, err := c.server.beginCall(d.DefLoc)
	if respErr != nil {
		respErr = errs.Convert(err)
		return
	}

	// Run the request in a different goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		reqObj, beginErr := c.server.beginRequest(c.ctx, &beginRequestParams{
			Type:     model.RPCCall,
			Service:  d.Service,
			Endpoint: d.Endpoint,
			DefLoc:   d.DefLoc,

			Path:         path,
			PathSegments: params,
			Payload:      d.ReqUserPayload(req),
			Inputs:       inputs,
			RPCDesc:      d.rpcDesc(),

			SpanID: call.SpanID,
		})
		if beginErr != nil {
			respErr = errs.B().Code(errs.Internal).Msg("internal error").Err()
			return
		}

		if err := d.validate(req); err != nil {
			respErr = err
			return
		}

		ec := c.server.newExecContext(c.ctx, params, model.AuthInfo{reqObj.UID, reqObj.AuthData})
		r, httpStatus, rpcErr := d.executeEndpoint(ec, func(mwReq middleware.Request) middleware.Response {
			return d.invokeHandlerNonRaw(mwReq, req)
		})

		if rpcErr == nil {
			r, rpcErr = d.CloneResp(r)
			if rpcErr != nil {
				// only override the http status if an error occurred trying to clone the response
				httpStatus = errs.HTTPStatus(errs.Convert(rpcErr))
			}
		}
		if rpcErr != nil {
			respErr = errs.RoundTrip(rpcErr)
			c.server.finishRequest(nil, respErr, httpStatus)
		} else {
			resp, respErr = r, nil
			output, _ := d.SerializeResp(c.server.json, r)
			c.server.finishRequest(output, nil, httpStatus)
		}
	}()
	<-done

	c.server.finishCall(call, respErr)
	return
}

// validate validates the request, and returns a validation error on failure.
// If the user payload does not implement Validator, it returns nil.
func (d *Desc[Req, Resp]) validate(req Req) error {
	return runValidate(d.ReqUserPayload(req))
}

// runValidate validates the request, and returns a validation error on failure.
// If the user payload does not implement Validator, it returns nil.
func runValidate(userPayload any) error {
	if v, ok := userPayload.(Validator); ok {
		if err := v.Validate(); err != nil {
			// If we already have an *errs.Error, return it directly.
			if _, ok := err.(*errs.Error); ok {
				return err
			}
			return errs.WrapCode(err, errs.InvalidArgument, "validation failed")
		}
	}
	return nil
}

// rpcDesc returns the RPC description for this endpoint,
// computing and caching the first time it's called.
func (d *Desc[Req, Resp]) rpcDesc() *model.RPCDesc {
	d.rpcDescOnce.Do(func() {
		var reqTyp Req
		desc := &model.RPCDesc{
			Service:     d.Service,
			Endpoint:    d.Endpoint,
			Raw:         d.Raw,
			RequestType: reflect.TypeOf(reqTyp),
		}

		if !isVoid[Resp]() {
			var typ Resp
			desc.ResponseType = reflect.TypeOf(typ)
		}
		d.cachedRPCDesc = desc
	})
	return d.cachedRPCDesc
}
