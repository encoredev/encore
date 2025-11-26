package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	jsoniter "github.com/json-iterator/go"

	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/exported/stack"
	"encore.dev/appruntime/shared/cloudtrace"
	"encore.dev/beta/errs"
	"encore.dev/internal/platformauth"
)

type AuthHandlerDesc[Params any] struct {
	// Service and Endpoint name the auth handler this description is for.
	Service     string
	SvcNum      uint16
	Endpoint    string
	DefLoc      uint32
	HasAuthData bool // whether the handler returns custom auth data

	DecodeAuth  func(*http.Request) (Params, error)
	AuthHandler func(context.Context, Params) (model.AuthInfo, error)

	rpcDescOnce   sync.Once
	cachedRPCDesc *model.RPCDesc
}

// AuthHandler is an interface that is either implemented by [AuthHandlerDesc[Params]] or [remoteAuthHandler]
// depending on whether the auth handler is hosted in this container or not.
type AuthHandler interface {
	// Authenticate authenticates the request.
	// If err != nil it should be written back as the response.
	Authenticate(IncomingContext) (model.AuthInfo, error)

	// HostedByService returns the name of the service that hosts the auth handler.
	HostedByService() string

	// ParseAuthData parses the auth data from the request and returns nil if successful.
	// and data is present, otherwise it returns an error.
	ParseAuthData(c IncomingContext) error
}

func (d *AuthHandlerDesc[Params]) Authenticate(c IncomingContext) (model.AuthInfo, error) {
	param, err := d.DecodeAuth(c.req)
	var info model.AuthInfo
	if err != nil {
		return model.AuthInfo{}, err
	}

	done := make(chan struct{})
	call, err := c.server.beginAuth(d.DefLoc)
	if err != nil {
		return model.AuthInfo{}, err
	}

	var authErr error
	go func() {
		defer close(done)
		_, authErr = c.server.beginRequest(c.req.Context(), &beginRequestParams{
			TraceID:       c.callMeta.TraceID,
			ParentSpanID:  c.callMeta.ParentSpanID,
			ParentSampled: c.callMeta.TraceSampled,
			SpanID:        call.SpanID,
			DefLoc:        d.DefLoc,
			Type:          model.AuthHandler,
			Data: &model.RPCData{
				Desc:               d.rpcDesc(),
				NonRawPayload:      d.marshalParams(c.server.json, param),
				TypedPayload:       param,
				RequestHeaders:     c.req.Header,
				FromEncorePlatform: platformauth.IsEncorePlatformRequest(c.req.Context()),
			},
			ExtCorrelationID:    clampTo64Chars(c.req.Header.Get("X-Correlation-ID")),
			AdditionalLogFields: cloudtrace.StructuredLogFields(c.req),
		})
		if authErr != nil {
			return
		}
		defer func() {
			if err2 := recover(); err2 != nil {
				panicStack := stack.Build(0)
				authErr = errs.B().Code(errs.Internal).Meta("panic_stack", panicStack).Msgf(
					"auth handler panicked: %v", err2).Err()
				c.server.finishRequest(newErrResp(authErr, 0))
			}
		}()

		if err := runValidate(param); err != nil {
			authErr = err
			c.server.finishRequest(newErrResp(authErr, 0))
			return
		}

		info, authErr = d.AuthHandler(c.req.Context(), param)

		if authErr != nil {
			authErr = errs.RoundTrip(authErr)
			c.server.finishRequest(newErrResp(authErr, 0))
		} else {
			resp := d.newAuthResp(info, authErr, c.server.json)
			c.server.finishRequest(resp)
		}
	}()
	<-done

	return info, authErr
}

func (d *AuthHandlerDesc[Params]) HostedByService() string {
	return d.Service
}

func (d *AuthHandlerDesc[Params]) ParseAuthData(c IncomingContext) error {
	_, err := d.DecodeAuth(c.req)
	return err
}

// runAuthHandler runs the auth handler, if provided.
// It reports whether to proceed with calling the handler.
func (s *Server) runAuthHandler(h Handler, c IncomingContext) (info model.AuthInfo, proceed bool) {
	requiresAuth := h.AccessType() == RequiresAuth
	if s.authHandler == nil {
		if requiresAuth {
			panic(fmt.Sprintf("internal error: API %s.%s requires auth but no auth handler set",
				h.ServiceName(), h.EndpointName()))
		}
		return model.AuthInfo{}, true
	}

	// If this is a service to service call, we use the existing auth info.
	if c.callMeta.IsServiceToService() {
		if c.auth.UID == "" && requiresAuth {
			// Unless there isn't some and we need it, in which case we error.
			err := errs.B().Code(errs.Unauthenticated).Msg("no auth info provided").Err()
			returnError(c, err, 0, nil)
			return model.AuthInfo{}, false
		}

		return c.auth, true
	}

	var err error
	info, err = s.authHandler.Authenticate(c)
	if err != nil {
		// If the auth handler returned Unauthenticated and the endpoint doesn't actually require auth,
		// continue as if no auth information was provided.
		if errs.Code(err) == errs.Unauthenticated && !requiresAuth {
			return model.AuthInfo{}, true
		} else {
			returnError(c, err, 0, nil)
			return model.AuthInfo{}, false
		}
	}

	return info, true
}

// rpcDesc returns the RPC description for this endpoint,
// computing and caching the first time it's called.
func (d *AuthHandlerDesc[Params]) rpcDesc() *model.RPCDesc {
	d.rpcDescOnce.Do(func() {
		desc := &model.RPCDesc{
			Service:     d.Service,
			SvcNum:      d.SvcNum,
			Endpoint:    d.Endpoint,
			AuthHandler: true,
			Raw:         false,

			// TODO would be nice to support these for auth handlers in the future.
			RequestType:  nil,
			ResponseType: nil,
			Tags:         nil,

			Exposed:      true,
			AuthRequired: false,
		}
		d.cachedRPCDesc = desc
	})
	return d.cachedRPCDesc
}

func (d *AuthHandlerDesc[Params]) marshalParams(json jsoniter.API, p Params) []byte {
	data, _ := json.Marshal(p)
	return data
}

// newAuthResp returns an *model.Response for an auth response.
func (d *AuthHandlerDesc[Params]) newAuthResp(info model.AuthInfo, authErr error, json jsoniter.API) *model.Response {
	var payload []byte
	if d.HasAuthData {
		payload = marshalParams(json, info.UserData)
	}

	return &model.Response{
		HTTPStatus:   errs.HTTPStatus(authErr),
		Err:          authErr,
		AuthUID:      info.UID,
		TypedPayload: info.UserData,
		Payload:      payload,
	}
}
