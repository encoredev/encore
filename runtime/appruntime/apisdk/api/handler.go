package api

import (
	"context"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"sync"

	jsoniter "github.com/json-iterator/go"

	encore "encore.dev"
	"encore.dev/appruntime/exported/model"
	"encore.dev/beta/errs"
	"encore.dev/internal/platformauth"
	"encore.dev/middleware"
)

// NamedParams are named path parameters.
type NamedParams = model.PathParams

// UnnamedParams are unnamed parameters from an incoming request.
type UnnamedParams []string

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
	// SvcNum is the 1-based index into the list of services.
	SvcNum uint16

	// Service and Endpoint name the API this description is for.
	Service  string
	Endpoint string

	Methods []string
	Path    string
	RawPath string
	DefLoc  int32

	// PathParamNames are the names of the path params, in order.
	PathParamNames []string

	// Access describes the access type for this API.
	Access Access

	// If raw is true, RawHandler is set and AppHandler and EncodeResp are nil.
	Raw bool

	// If Fallback is true, the handler is a fallback handler
	// for when other routes don't match.
	Fallback bool

	DecodeReq      func(*http.Request, UnnamedParams, jsoniter.API) (Req, UnnamedParams, error)
	CloneReq       func(Req) (Req, error)
	ReqPath        func(Req) (path string, params UnnamedParams, err error)
	ReqUserPayload func(Req) any

	AppHandler func(context.Context, Req) (Resp, error)
	RawHandler func(http.ResponseWriter, *http.Request)

	EncodeResp func(http.ResponseWriter, jsoniter.API, Resp) error
	CloneResp  func(Resp) (Resp, error)

	// GlobalMiddlewareIDs is the ordered list of global middleware IDs
	// to invoke before calling the API handler.
	GlobalMiddlewareIDs []string

	// ServiceMiddleware is the ordered list of middleware to invoke before
	// calling the API handler.
	ServiceMiddleware []*Middleware

	rpcDescOnce   sync.Once
	cachedRPCDesc *model.RPCDesc
}

func (d *Desc[Req, Resp]) AccessType() Access     { return d.Access }
func (d *Desc[Req, Resp]) ServiceName() string    { return d.Service }
func (d *Desc[Req, Resp]) EndpointName() string   { return d.Endpoint }
func (d *Desc[Req, Resp]) HTTPMethods() []string  { return d.Methods }
func (d *Desc[Req, Resp]) SemanticPath() string   { return d.Path }
func (d *Desc[Req, Resp]) HTTPRouterPath() string { return d.RawPath }
func (d *Desc[Req, Resp]) IsFallback() bool       { return d.Fallback }

func (d *Desc[Req, Resp]) Handle(c IncomingContext) {
	if d.Raw {
		c.capturer = newRawRequestBodyCapturer(c.req)
		c.req.Body = c.capturer
		defer c.capturer.Dispose()
	}

	reqData, beginErr := d.begin(c)
	if beginErr != nil {
		errs.HTTPError(c.w, beginErr)
		return
	}

	resp, respData := d.handleIncoming(c, reqData)
	if resp.Err != nil {
		c.server.finishRequest(resp)

		// If the endpoint is raw it has already written its response;
		// don't write another.
		if !d.Raw {
			errs.HTTPErrorWithCode(c.w, resp.Err, resp.HTTPStatus)
		}
		return
	}

	if !d.Raw {
		c.w.Header().Set("Content-Type", "application/json")
		c.w.Header().Set("X-Content-Type-Options", "nosniff")
		resp.Err = d.EncodeResp(c.w, c.server.json, respData)
	}
	c.server.finishRequest(resp)
}

func (d *Desc[Req, Resp]) begin(c IncomingContext) (reqData Req, beginErr error) {
	reqData, params, decodeErr := d.DecodeReq(c.req, c.ps, c.server.json)

	if d.Access == RequiresAuth && c.auth.UID == "" {
		beginErr = errs.B().
			Code(errs.Unauthenticated).
			Meta("service", d.Service, "endpoint", d.Endpoint).
			Msg("endpoint requires auth but none provided").
			Err()
		return
	}

	// Only compute inputs and payload if we have valid reqData.
	var payload any
	var nonRawPayload []byte
	if decodeErr == nil {
		payload = d.ReqUserPayload(reqData)
		if !d.Raw {
			nonRawPayload = marshalParams(c.server.json, payload)
		}
	}

	_, err := c.server.beginRequest(c.ctx, &beginRequestParams{
		Type:    model.RPCCall,
		DefLoc:  d.DefLoc,
		TraceID: c.traceID,

		Data: &model.RPCData{
			Desc:               d.rpcDesc(),
			HTTPMethod:         c.req.Method,
			Path:               c.req.URL.Path,
			PathParams:         d.toNamedParams(params),
			TypedPayload:       payload,
			NonRawPayload:      nonRawPayload,
			UserID:             c.auth.UID,
			AuthData:           c.auth.UserData,
			RequestHeaders:     c.req.Header,
			FromEncorePlatform: platformauth.IsEncorePlatformRequest(c.req.Context()),
		},

		ExtRequestID:     clampTo64Chars(c.req.Header.Get("X-Request-ID")),
		ExtCorrelationID: clampTo64Chars(c.req.Header.Get("X-Correlation-ID")),
	})
	if err != nil {
		beginErr = errs.B().Code(errs.Internal).Msg("internal error").Err()
		return
	}

	// If we fail after having begun the request, mark it as completed.
	defer func() {
		if beginErr != nil {
			c.server.finishRequest(newErrResp(beginErr, 0))
		}
	}()

	if decodeErr != nil {
		beginErr = errs.WrapCode(decodeErr, errs.InvalidArgument, "decode request")
		return
	}

	return reqData, nil
}

// handleIncoming executes the given handler, running middleware in the process.
func (d *Desc[Req, Resp]) handleIncoming(c IncomingContext, reqData Req) (resp *model.Response, respData Resp) {
	if err := d.validate(reqData); err != nil {
		return newErrResp(err, 0), respData
	}

	var respCapturer *rawResponseCapturer

	invokeHandler := func(mwReq middleware.Request) (mwResp middleware.Response) {
		if d.Raw {
			respCapturer = newRawResponseCapturer(c.w, c.req)
			return d.invokeHandlerRaw(mwReq, c, respCapturer)
		} else {
			return d.invokeHandlerNonRaw(mwReq, reqData)
		}
	}

	respData, httpStatus, err := d.executeEndpoint(c.execContext, invokeHandler)

	resp = newResp(respData, httpStatus, err, d.Raw, c.capturer, respCapturer, c.server.json)
	return resp, respData
}

// executeEndpoint executes the given handler, running middleware in the process.
func (d *Desc[Req, Resp]) executeEndpoint(c execContext, invokeHandler func(middleware.Request) middleware.Response) (resp Resp, httpStatus int, respErr error) {
	var counter int
	var nextFn middleware.Next

	var (
		globalMiddleware = c.server.getGlobalMiddleware(d.GlobalMiddlewareIDs)
		svcMiddleware    = d.ServiceMiddleware

		numGlobalMiddleware  = len(globalMiddleware)
		numServiceMiddleware = len(svcMiddleware)
		numTotalMiddleware   = numGlobalMiddleware + numServiceMiddleware
	)

	nextFn = func(req middleware.Request) (resp middleware.Response) {
		// Ensure the HTTP status code is correctly set in the response
		defer func() {
			// If no explicit HTTP status has been set, then we use the default for the type of error
			// or if Err is nil, we'll set 200
			if resp.HTTPStatus == 0 {
				resp.HTTPStatus = errs.HTTPStatus(resp.Err)
			}
		}()

		idx := counter
		counter++

		switch {
		case idx < numTotalMiddleware:
			var mw *Middleware
			if idx < numGlobalMiddleware {
				mw = globalMiddleware[idx]
			} else {
				mw = svcMiddleware[idx-numGlobalMiddleware]
			}

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

		case idx == numTotalMiddleware:
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
		return resp, mwResp.HTTPStatus, mwResp.Err
	} else {
		if resp, ok := mwResp.Payload.(Resp); ok || isVoid[Resp]() {
			return resp, mwResp.HTTPStatus, mwResp.Err
		}
	}

	return resp, 500, errs.B().Code(errs.Internal).Msgf(
		"invalid middleware: cannot return payload of type %T for endpoint %s.%s (expected type %T)",
		mwResp.Payload, d.Service, d.Endpoint, resp,
	).Err()
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
func (d *Desc[Req, Resp]) invokeHandlerRaw(mwReq middleware.Request, c IncomingContext, capturer *rawResponseCapturer) (mwResp middleware.Response) {
	if !d.Raw {
		panic("invokeHandlerRaw called on non-Raw endpoint")
	}

	// Middleware can override the context, so check if the context is different
	// and if so change the request context.
	httpReq := c.req
	if ctx := mwReq.Context(); ctx != c.req.Context() {
		httpReq = httpReq.WithContext(ctx)
	}

	capturer.InvokeHandler(http.HandlerFunc(d.RawHandler), httpReq)

	if capturer.Code >= 400 {
		statusText := http.StatusText(capturer.Code)
		if statusText == "" {
			statusText = "HTTP " + strconv.Itoa(capturer.Code)
		}
		mwResp.Err = errs.B().Code(errs.HTTPStatusToCode(capturer.Code)).Msg(statusText).Err()
	}

	mwResp.HTTPStatus = capturer.Code
	return mwResp
}

func (d *Desc[Req, Resp]) toNamedParams(ps UnnamedParams) NamedParams {
	named := make(NamedParams, len(ps))
	for i, p := range ps {
		named[i].Name = d.PathParamNames[i]
		named[i].Value = p
	}
	return named
}

type CallContext struct {
	ctx    context.Context
	server *Server
}

func (d *Desc[Req, Resp]) Call(c CallContext, req Req) (respData Resp, respErr error) {
	// TODO: we don't currently support service-to-service calls of raw endpoints.
	// To fix this we need to improve our request serialization and DI support to
	// separate the signature for outgoing calls versus handlers.
	if d.Raw {
		respErr = errs.B().Code(errs.Internal).Msg("internal encore error: cannot call raw endpoints in service-to-service calls").Err()
		return
	}

	req, err := d.CloneReq(req)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to clone request")
		respErr = errs.Convert(err)
		return
	}

	path, params, err := d.ReqPath(req)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to compute request path")
		respErr = errs.Convert(err)
		return
	}

	call, err := c.server.beginCall(d.DefLoc)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to begin call")
		respErr = errs.Convert(err)
		return
	}

	// Run the request in a different goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Default to GET if there are no methods available (or it's a wildcard)
		httpMethod := "GET"
		if len(d.Methods) > 0 && d.Methods[0] != "*" {
			httpMethod = d.Methods[0]
		}

		userPayload := d.ReqUserPayload(req)
		var nonRawPayload []byte
		if !d.Raw {
			nonRawPayload = marshalParams(c.server.json, userPayload)
		}

		reqObj, beginErr := c.server.beginRequest(c.ctx, &beginRequestParams{
			Type:   model.RPCCall,
			DefLoc: d.DefLoc,

			Data: &model.RPCData{
				HTTPMethod:    httpMethod,
				Path:          path,
				PathParams:    d.toNamedParams(params),
				TypedPayload:  userPayload,
				Desc:          d.rpcDesc(),
				NonRawPayload: nonRawPayload,

				FromEncorePlatform: false,
				RequestHeaders:     nil, // not set right now for internal requests
			},

			SpanID: call.SpanID,
		})
		if beginErr != nil {
			respErr = errs.B().Cause(beginErr).Code(errs.Internal).Msg("internal error").Err()
			return
		}

		if err := d.validate(req); err != nil {
			respErr = err
			return
		}

		ec := c.server.newExecContext(c.ctx, params, reqObj.TraceID, model.AuthInfo{reqObj.RPCData.UserID, reqObj.RPCData.AuthData})
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
			c.server.finishRequest(newErrResp(respErr, httpStatus))
		} else {
			respData, respErr = r, nil
			// Always nil for now since we don't support raw endpoints in service-to-service calls.
			var (
				reqCapture  *rawRequestBodyCapturer
				respCapture *rawResponseCapturer
			)
			modelResp := newResp(respData, httpStatus, respErr, d.Raw, reqCapture, respCapture, c.server.json)
			c.server.finishRequest(modelResp)
		}
	}()
	<-done

	c.server.rootLogger.Err(respErr).Msg("call failed")
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
			SvcNum:      d.SvcNum,
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

func marshalParams[Resp any](json jsoniter.API, resp Resp) []byte {
	if isVoid[Resp]() {
		return nil
	}
	data, _ := json.Marshal(resp)
	return data
}

// newResp returns an *model.Response for a response.
func newResp[Resp any](respData Resp, httpStatus int, err error, isRaw bool,
	reqCapture *rawRequestBodyCapturer, respCapture *rawResponseCapturer, json jsoniter.API,
) *model.Response {
	resp := &model.Response{
		HTTPStatus: httpStatus,
		Err:        err,
	}

	if isRaw {
		if reqCapture != nil {
			resp.RawRequestPayload, resp.RawRequestPayloadOverflowed = reqCapture.FinishCapturing()
		}
		if respCapture != nil {
			resp.RawResponseHeaders = respCapture.Header
			resp.RawResponsePayload, resp.RawResponsePayloadOverflowed = respCapture.FinishCapturing()
		}
	} else {
		resp.TypedPayload = respData
		resp.Payload = marshalParams(json, respData)
	}
	return resp
}

// newErrResp returns an *model.Response for an error.
// If httpStatus is 0 it's inferred from the error.
func newErrResp(err error, httpStatus int) *model.Response {
	if httpStatus == 0 {
		httpStatus = errs.HTTPStatus(err)
	}
	return &model.Response{
		HTTPStatus: httpStatus,
		Err:        err,
	}
}
