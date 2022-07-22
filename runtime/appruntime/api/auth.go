package api

import (
	"context"
	"fmt"
	"net/http"

	"encore.dev/appruntime/model"
	"encore.dev/beta/errs"
)

type AuthHandlerDesc[Params Serializable] struct {
	// Service and Endpoint name the auth handler this description is for.
	Service     string
	Endpoint    string
	DefLoc      int32
	HasAuthData bool // whether the handler returns custom auth data

	DecodeAuth  func(*http.Request) (Params, error)
	AuthHandler func(ctx context.Context, p Params) (model.AuthInfo, error)
}

type AuthHandler interface {
	Authenticate(Context) (model.AuthInfo, error)
}

// Authenticate authenticates the request.
// If err != nil it should be written back as the response.
func (d *AuthHandlerDesc[Params]) Authenticate(c Context) (model.AuthInfo, error) {
	param, err := d.DecodeAuth(c.req)
	var info model.AuthInfo
	if err != nil {
		return model.AuthInfo{}, err
	}

	done := make(chan struct{})
	call, err := c.server.beginAuth()
	if err != nil {
		return model.AuthInfo{}, err
	}

	var authErr error
	go func() {
		defer close(done)
		inputs, _ := param.Serialize(c.server.json)
		authErr = c.server.beginRequest(c.req.Context(), &beginRequestParams{
			SpanID:   call.SpanID,
			Service:  d.Service,
			Endpoint: d.Endpoint,
			DefLoc:   d.DefLoc,
			Inputs:   inputs,
			Type:     model.AuthHandler,
		})
		if authErr != nil {
			return
		}
		defer func() {
			if err2 := recover(); err2 != nil {
				authErr = errs.B().Code(errs.Internal).Msgf("auth handler panicked: %v", err2).Err()
				c.server.finishRequest(nil, authErr, 0)
			}
		}()
		info, authErr = d.AuthHandler(c.req.Context(), param)

		if authErr != nil {
			authErr = errs.RoundTrip(authErr)
			c.server.finishRequest(nil, authErr, 0)
		} else {
			output, _ := info.Serialize(c.server.json)
			c.server.finishRequest(output, nil, 0)
		}
	}()
	<-done

	c.server.finishAuth(call, info.UID, authErr)
	return info, authErr
}

// runAuthHandler runs the auth handler, if provided.
// It reports whether to proceed with calling the handler.
func (s *Server) runAuthHandler(h Handler, c Context) (info model.AuthInfo, proceed bool) {
	requiresAuth := h.AccessType() == RequiresAuth
	if s.authHandler == nil {
		if requiresAuth {
			panic(fmt.Sprintf("internal error: API %s.%s requires auth but no auth handler set",
				h.ServiceName(), h.EndpointName()))
		}
		return model.AuthInfo{}, true
	}

	var err error
	info, err = s.authHandler.Authenticate(c)
	if err != nil {
		// If the auth handler returned Unauthenticated and the endpoint doesn't actually require auth,
		// continue as if no auth information was provided.
		if errs.Code(err) == errs.Unauthenticated && !requiresAuth {
			return model.AuthInfo{}, true
		} else {
			errs.HTTPError(c.w, err)
			return model.AuthInfo{}, false
		}
	}

	return info, true
}
