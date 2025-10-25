package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"sync"

	jsoniter "github.com/json-iterator/go"

	encore "encore.dev"
	"encore.dev/appruntime/apisdk/api/errmarshalling"
	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/shared/cfgutil"
	"encore.dev/appruntime/shared/cloudtrace"
	"encore.dev/appruntime/shared/jsonapi"
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
	DefLoc  uint32

	// PathParamNames are the names of the path params, in order.
	PathParamNames []string

	// Tags are the tags for this API, excluding the "tag:" prefix.
	Tags []string

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

	EncodeResp func(http.ResponseWriter, jsoniter.API, Resp, int) error
	CloneResp  func(Resp) (Resp, error)

	// EncodeExternalReq encodes a request, writing the payload to the stream
	// and headers and query strings to the returned maps.
	EncodeExternalReq func(Req, *jsoniter.Stream) (http.Header, url.Values, error)

	// DecodeExternalResp decodes the response, reading the payload into the response object.
	DecodeExternalResp func(*http.Response, jsoniter.API) (Resp, error)

	// GlobalMiddlewareIDs is the ordered list of global middleware IDs
	// to invoke before calling the API handler.
	GlobalMiddlewareIDs []string

	// ServiceMiddleware is the ordered list of middleware to invoke before
	// calling the API handler.
	ServiceMiddleware []*Middleware

	rpcDescOnce   sync.Once
	cachedRPCDesc *model.RPCDesc

	mockCacheMu   sync.RWMutex
	mockObjCache  map[any]reflectedAPIMethod[Req, Resp]    // map of object to reflected method
	mockFuncCache map[uint64]reflectedAPIMethod[Req, Resp] // map of model.ApiMock.ID to reflected method
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

	// If this is an internal encore-to-encore call, we need to verify the caller is allowed to make this call.
	if c.callMeta.IsServiceToService() {
		t := transport.HTTPRequest(c.req)
		targetAPI, _ := t.ReadMeta(calleeMetaName)

		if targetAPI != fmt.Sprintf("%s.%s", d.Service, d.Endpoint) {
			returnError(c, errs.B().Code(errs.PermissionDenied).Msg("internal call auth did not align with API").Err(), 0, nil)
			return
		}
	}

	reqData, beginErr := d.begin(c)
	if beginErr != nil {
		returnError(c, beginErr, 0, nil)
		return
	}

	resp, respData := d.handleIncoming(c, reqData)
	if resp.Err != nil {
		c.server.finishRequest(resp)

		// If the endpoint is raw it has already written its response;
		// don't write another.
		if !d.Raw {
			returnError(c, resp.Err, resp.HTTPStatus, resp.Headers)
		}
		return
	}

	if !d.Raw {
		// Apply any custom headers from middleware
		for key, values := range resp.Headers {
			for _, value := range values {
				c.w.Header().Add(key, value)
			}
		}

		c.w.Header().Set("Content-Type", "application/json")
		c.w.Header().Set("X-Content-Type-Options", "nosniff")
		resp.Err = d.EncodeResp(c.w, c.server.json, respData, resp.HTTPStatus)
	}
	c.server.finishRequest(resp)
}

// returnError is a helper function which will return an error to the client when we handle
// an incoming request.
//
// If the requests is an internal service to service request, we will return an error
// in a full unabridged format, allowing the calling code to unmarshal the error without
// loss of information.
//
// If the request is an external request, we will return a more user friendly error
// message, which will be more suitable for display to the end user and not include
// any internal details.
//
// If statusCodeToUse is 0, we will use the default status code for the error using
// the [errs] package.
//
// headers will be applied to the response if provided.
func returnError(c IncomingContext, err error, statusCodeToUse int, headers http.Header) {
	// Apply any custom headers from middleware
	for key, values := range headers {
		for _, value := range values {
			c.w.Header().Add(key, value)
		}
	}

	if c.callMeta.PrivateAPIAccess() {
		// If this is an internal service to service call, we want to return the full error
		// we'll add a header to the response to indicate that the error is a full error
		// and the calling code can use this to determine how to unmarshal the response object.
		c.w.Header().Set("X-Encore-Full-Error", "1")

		c.w.Header().Set("Content-Type", "application/json")
		c.w.Header().Set("X-Content-Type-Options", "nosniff")

		// Write the status code out
		if statusCodeToUse == 0 {
			statusCodeToUse = errs.HTTPStatus(err)
		}
		c.w.WriteHeader(statusCodeToUse)

		// Use our internal error marshalling package to marshal the error
		errBytes := errmarshalling.Marshal(err)
		_, _ = c.w.Write(errBytes)

	} else {
		errs.HTTPErrorWithCode(c.w, err, statusCodeToUse)
	}
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
		Type:          model.RPCCall,
		DefLoc:        d.DefLoc,
		TraceID:       c.callMeta.TraceID,
		SpanID:        c.callMeta.SpanID,
		ParentSpanID:  c.callMeta.ParentSpanID,
		CallerEventID: c.callMeta.ParentEventID,
		ParentSampled: c.callMeta.TraceSampled,

		Data: &model.RPCData{
			Desc:                 d.rpcDesc(),
			HTTPMethod:           c.req.Method,
			Path:                 c.req.URL.Path,
			PathParams:           d.toNamedParams(params),
			TypedPayload:         payload,
			NonRawPayload:        nonRawPayload,
			UserID:               c.auth.UID,
			AuthData:             c.auth.UserData,
			RequestHeaders:       headersWithHost(c.req),
			FromEncorePlatform:   platformauth.IsEncorePlatformRequest(c.req.Context()),
			ServiceToServiceCall: c.callMeta.IsServiceToService(),
		},

		ExtRequestID:        clampTo64Chars(c.req.Header.Get("X-Request-ID")),
		ExtCorrelationID:    clampTo64Chars(c.req.Header.Get("X-Correlation-ID")),
		AdditionalLogFields: cloudtrace.StructuredLogFields(c.req),
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
			return d.invokeHandlerNonRaw(mwReq, reqData, d.AppHandler)
		}
	}

	respData, httpStatus, headers, err := d.executeEndpoint(c.execContext, invokeHandler)

	resp = newRespWithHeaders(respData, httpStatus, err, headers, d.Raw, c.capturer, respCapturer, c.server.json)
	return resp, respData
}

// executeEndpoint executes the given handler, running middleware in the process.
func (d *Desc[Req, Resp]) executeEndpoint(c execContext, invokeHandler func(middleware.Request) middleware.Response) (resp Resp, httpStatus int, headers http.Header, respErr error) {
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
					panicStack := stack.BuildWithoutGoRuntime(2)
					resp.Err = errs.B().Code(errs.Internal).Stack(panicStack).Meta("panic_stack", panicStack).Msgf("panic executing middleware %s.%s: %v",
						mw.PkgName, mw.Name, e).Err()
					resp.HTTPStatus = 500
				}
			}()
			return mw.Invoke(req, nextFn)

		case idx == numTotalMiddleware:
			defer func() {
				// Catch handler panic
				if e := recover(); e != nil {
					panicStack := stack.BuildWithoutGoRuntime(2)
					resp.Err = errs.B().Code(errs.Internal).Stack(panicStack).Meta("panic_stack", panicStack).Msgf(
						"panic handling request: %v", e).Err()
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
		return resp, mwResp.HTTPStatus, mwResp.GetHeaders(), mwResp.Err
	} else {
		if resp, ok := mwResp.Payload.(Resp); ok || isVoid[Resp]() {
			return resp, mwResp.HTTPStatus, mwResp.GetHeaders(), mwResp.Err
		}
	}

	return resp, 500, mwResp.GetHeaders(), errs.B().Code(errs.Internal).Msgf(
		"invalid middleware: cannot return payload of type %T for endpoint %s.%s (expected type %T)",
		mwResp.Payload, d.Service, d.Endpoint, resp,
	).Err()
}

// invokeHandlerNonRaw invokes the handler for a regular (non-raw) endpoint. If the endpoint is raw, it panics.
func (d *Desc[Req, Resp]) invokeHandlerNonRaw(mwReq middleware.Request, reqData Req, handler func(context.Context, Req) (Resp, error)) (mwResp middleware.Response) {
	if d.Raw {
		panic("invokeHandlerNonRaw called on Raw endpoint")
	}

	handlerResp, handlerErr := handler(mwReq.Context(), reqData)
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
	// If we're inside a test, we need to check if the target service has been mocked
	// and if it has, we need to route the call to the mock, otherwise
	// we'll make an internal call to the API
	if c.server.static.Testing {
		if mockedAPI, found := c.server.testingMgr.GetAPIMock(d.Service, d.Endpoint); found && mockedAPI.Function != nil {
			function, err := d.getMockFunction(mockedAPI)
			if err != nil {
				return respData, errs.Wrap(err, "unable to call mocked API due to an issue with the mock")
			}
			return d.mockedCall(c, function, req, mockedAPI.RunMiddleware)
		} else if mockedService, found := c.server.testingMgr.GetServiceMock(d.Service); found && mockedService.Service != nil {
			method, err := d.getMockMethod(mockedService)
			if err != nil {
				return respData, errs.Wrap(err, "unable to call mocked API due to an issue with the mock")
			}
			return d.mockedCall(c, method, req, mockedService.RunMiddleware)
		} else {
			return d.internalCall(c, req)
		}
	}

	if cfgutil.IsHostedService(c.server.runtime, d.Service) {
		// If we're calling a hosted service, we can route via the
		// internal process
		return d.internalCall(c, req)
	}

	// Otherwise we need to route via the service discovery mechanism
	service, found := c.server.runtime.ServiceDiscovery[d.Service]
	if !found {
		// Any service we need to talk to should be in the service discovery map, if it is not
		// that implies the code is doing something unexpected and we should fail fast.
		return respData, errs.B().Code(errs.Internal).Meta("service", d.Service).Msg("no route to service found").Err()
	} else {
		return d.externalCall(c, service, req)
	}
}

// getMockMethod returns a reflected method for the given object, caching the result.
// so subsequent calls will be faster.
func (d *Desc[Req, Resp]) getMockMethod(svcMock model.ServiceMock) (reflectedAPIMethod[Req, Resp], error) {
	d.mockCacheMu.RLock()
	method, found := d.mockObjCache[svcMock.Service]
	d.mockCacheMu.RUnlock()

	if !found {
		d.mockCacheMu.Lock()
		defer d.mockCacheMu.Unlock()

		// Get a reflected value of the object
		val := reflect.ValueOf(svcMock.Service)
		if !val.IsValid() {
			return nil, errs.B().Code(errs.Internal).Msgf("object %T is not valid", svcMock).Err()
		}

		// Get the method
		// nosemgrep
		methodVal := val.MethodByName(d.Endpoint)
		if !methodVal.IsValid() {
			return nil, errs.B().Code(errs.Internal).Msgf("method %s not found on object %T", d.Endpoint, svcMock.Service).Err()
		}

		m, err := createReflectionCaller[Req, Resp](methodVal)
		if err != nil {
			return nil, errs.Wrap(err, "unable to create mock caller")
		}

		// Cache the method
		if len(d.mockObjCache) == 0 {
			d.mockObjCache = make(map[any]reflectedAPIMethod[Req, Resp])
		}
		d.mockObjCache[svcMock.Service] = m
		return m, nil
	}

	return method, nil
}

// getMockFunction returns a reflected method for the given function, caching the result.
// so subsequent calls will be faster.
func (d *Desc[Req, Resp]) getMockFunction(apiMock model.ApiMock) (reflectedAPIMethod[Req, Resp], error) {
	d.mockCacheMu.RLock()
	method, found := d.mockFuncCache[apiMock.ID]
	d.mockCacheMu.RUnlock()

	if !found {
		d.mockCacheMu.Lock()
		defer d.mockCacheMu.Unlock()

		// Get a reflected value of the object
		val := reflect.ValueOf(apiMock.Function)
		if !val.IsValid() {
			return nil, errs.B().Code(errs.Internal).Msgf("function %T is not valid", apiMock.Function).Err()
		}

		m, err := createReflectionCaller[Req, Resp](val)
		if err != nil {
			return nil, errs.Wrap(err, "unable to create mock caller")
		}

		// Cache the method
		if len(d.mockFuncCache) == 0 {
			d.mockFuncCache = make(map[uint64]reflectedAPIMethod[Req, Resp])
		}
		d.mockFuncCache[apiMock.ID] = m
		return m, nil
	}

	return method, nil
}

func (d *Desc[Req, Resp]) mockedCall(c CallContext, mock reflectedAPIMethod[Req, Resp], req Req, runMiddleware bool) (respData Resp, respErr error) {
	return d.runCall(c, req, true, func(ec execContext, req Req) (Resp, int, error) {
		// If we want to run middleware, use the same code path as internalCall but switch out the handler
		// to our mock.
		if runMiddleware {
			resp, status, _, err := d.executeEndpoint(ec, func(mwReq middleware.Request) (mwResp middleware.Response) {
				return d.invokeHandlerNonRaw(mwReq, req, mock)
			})
			return resp, status, err
		}

		respData, err := mock(ec.ctx, req)
		if err != nil {
			return respData, errs.HTTPStatus(err), err
		}
		return respData, 200, nil
	})
}

func (d *Desc[Req, Resp]) internalCall(c CallContext, req Req) (respData Resp, respErr error) {
	return d.runCall(c, req, false, func(ec execContext, req Req) (Resp, int, error) {
		resp, status, _, err := d.executeEndpoint(ec, func(mwReq middleware.Request) middleware.Response {
			return d.invokeHandlerNonRaw(mwReq, req, d.AppHandler)
		})
		return resp, status, err
	})
}

func (d *Desc[Req, Resp]) runCall(c CallContext, req Req, mocked bool, executor func(ec execContext, req Req) (Resp, int, error)) (respData Resp, respErr error) {
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

	call, meta, err := c.server.beginCall(c.ctx, d.Service, d.Endpoint, d.DefLoc)
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

		reqModel, beginErr := c.server.beginRequest(c.ctx, &beginRequestParams{
			Type:          model.RPCCall,
			DefLoc:        d.DefLoc,
			CallerEventID: call.StartEventID,

			Data: &model.RPCData{
				HTTPMethod:    httpMethod,
				Path:          path,
				PathParams:    d.toNamedParams(params),
				TypedPayload:  userPayload,
				Desc:          d.rpcDesc(),
				NonRawPayload: nonRawPayload,

				FromEncorePlatform:   false,
				RequestHeaders:       nil, // not set right now for internal requests
				ServiceToServiceCall: true,
				Mocked:               mocked,
			},
		})

		// Now round-trip any auth data that was set on the request
		// to emulate what happens in the HTTP case.
		if experiments.AuthDataRoundTrip.Enabled(c.server.experiments) && reqModel.RPCData.AuthData != nil {
			jsonBytes, err := jsonapi.Default.Marshal(reqModel.RPCData.AuthData)
			if err != nil {
				c.server.rootLogger.Err(err).Msg("unable to marshal auth data")
				respErr = errs.B().Cause(err).Code(errs.Internal).Msg("internal error").Err()
				return
			}

			reqModel.RPCData.AuthData = newAuthDataObj()
			if err := jsonapi.Default.Unmarshal(jsonBytes, reqModel.RPCData.AuthData); err != nil {
				c.server.rootLogger.Err(err).Msg("unable to unmarshal auth data")
				respErr = errs.B().Cause(err).Code(errs.Internal).Msg("internal error").Err()
				return
			}
		}

		if beginErr != nil {
			respErr = errs.B().Cause(beginErr).Code(errs.Internal).Msg("internal error").Err()
			return
		}

		if err := d.validate(req); err != nil {
			respErr = err
			return
		}

		ec := c.server.newExecContext(c.ctx, params, meta)
		r, httpStatus, rpcErr := executor(ec, req)

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

	if respErr != nil {
		c.server.rootLogger.Err(respErr).Msg("call failed")
	}
	c.server.finishCall(call, respErr)
	return
}

func (d *Desc[Req, Resp]) externalCall(c CallContext, service config.Service, req Req) (respData Resp, respErr error) {
	// TODO: we don't currently support service-to-service calls of raw endpoints.
	// To fix this we need to improve our request serialization and DI support to
	// separate the signature for outgoing calls versus handlers.
	if d.Raw {
		respErr = errs.B().Code(errs.Internal).Msg("internal encore error: cannot call raw endpoints in service-to-service calls").Err()
		return
	}

	// Lookup the service
	if service.Protocol != config.Http {
		// For now we only support HTTP services
		respErr = errs.B().Code(errs.Internal).Msg("internal encore error: unsupported service protocol").Err()
		return
	}

	path, _, err := d.ReqPath(req)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to compute request path")
		respErr = errs.Convert(err)
		return
	}

	// Default to POST if there are no methods available (or it's a wildcard)
	httpMethod := "POST"
	if len(d.Methods) > 0 && d.Methods[0] != "*" {
		httpMethod = d.Methods[0]
	}

	// Encode the request payload
	var buf bytes.Buffer
	stream := c.server.json.BorrowStream(&buf)
	header, queryString, err := d.EncodeExternalReq(req, stream)
	if err2 := stream.Flush(); err == nil {
		err = err2
	}
	c.server.json.ReturnStream(stream)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to marshal request")
		respErr = errs.Convert(err)
		return
	}

	reqURL := service.URL + path
	if len(queryString) > 0 {
		reqURL += "?" + queryString.Encode()
	}
	httpReq, err := http.NewRequestWithContext(c.ctx, httpMethod, reqURL, &buf)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to create HTTP request")
		respErr = errs.Convert(err)
		return
	}
	for key, val := range header {
		httpReq.Header[key] = val
	}
	reqTransport := transport.HTTPRequest(httpReq)

	// Set the name of the API we want to call
	reqTransport.SetMeta(calleeMetaName, fmt.Sprintf("%s.%s", d.Service, d.Endpoint))

	call, meta, err := c.server.beginCall(c.ctx, d.Service, d.Endpoint, d.DefLoc)
	if err != nil {
		c.server.rootLogger.Err(err).Msg("unable to begin call")
		respErr = errs.Convert(err)
		return
	}

	if err := meta.AddToRequest(c.server, service, reqTransport); err != nil {
		c.server.rootLogger.Err(err).Msg("unable to add metadata to request")
		respErr = errs.Convert(err)
		return
	}

	respData, respErr = (func() (resp Resp, err error) {
		httpResp, err := c.server.httpClient.Do(httpReq)
		if err != nil {
			return resp, err
		}
		defer func() { _ = httpResp.Body.Close() }()

		if httpResp.StatusCode >= 400 {
			return resp, unmarshalErrorResponse(httpResp)
		}

		return d.DecodeExternalResp(httpResp, c.server.json)
	})()

	if respErr != nil {
		c.server.rootLogger.Err(respErr).Str("target", fmt.Sprintf("%s.%s", d.Service, d.Endpoint)).Str("url", reqURL).Msg("call failed")
	}

	c.server.finishCall(call, respErr)
	return
}

// unmarshalErrorResponse unmarshals an error response from an HTTP response.
//
// If the error sent back from the server was an Encore encoded error, we unmashal that to restore the original
// error types. Otherwise, we return a internal server generic error.
func unmarshalErrorResponse(httpResp *http.Response) error {
	if httpResp.Header.Get("X-Encore-Full-Error") == "1" {
		errBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return errs.B().Code(errs.Internal).Msg("request failed: unable to read response").Err()
		}

		respErr, marshallingErr := errmarshalling.Unmarshal(errBytes)
		if marshallingErr != nil {
			return errs.B().Code(errs.Internal).Cause(err).Msg("request failed: unable to unmarshal response").Err()
		}

		return respErr
	} else {
		bodyBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return errs.B().Code(errs.Internal).Msg("request failed: unable to read response").Err()
		}

		if len(bodyBytes) == 0 {
			return errs.B().Code(errs.Internal).Msgf("request failed: status %s", httpResp.Status).Err()
		} else {
			return errs.B().Code(errs.Internal).Msgf("request failed: status %s: %s", httpResp.Status, string(bodyBytes)).Err()
		}
	}
}

// headersWithHost clones the request headers and adds the Host header from req.Host.
// This is needed because Go stores the Host header in req.Host, not in req.Header.
func headersWithHost(req *http.Request) http.Header {
	var headers http.Header
	if req.Header != nil {
		headers = maps.Clone(req.Header)
	} else {
		headers = make(http.Header)
	}
	if req.Host != "" {
		headers.Set("Host", req.Host)
	}
	return headers
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
			Service:      d.Service,
			SvcNum:       d.SvcNum,
			Endpoint:     d.Endpoint,
			Raw:          d.Raw,
			RequestType:  reflect.TypeOf(reqTyp),
			Tags:         d.Tags,
			Exposed:      d.Access == Public || d.Access == RequiresAuth,
			AuthRequired: d.Access == RequiresAuth,
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
	return newRespWithHeaders(respData, httpStatus, err, nil, isRaw, reqCapture, respCapture, json)
}

// newRespWithHeaders returns an *model.Response for a response with custom headers.
func newRespWithHeaders[Resp any](respData Resp, httpStatus int, err error, headers http.Header, isRaw bool,
	reqCapture *rawRequestBodyCapturer, respCapture *rawResponseCapturer, json jsoniter.API,
) *model.Response {
	resp := &model.Response{
		HTTPStatus: httpStatus,
		Err:        err,
		Headers:    headers,
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
