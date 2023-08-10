package api

import (
	"maps"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/beta/errs"
)

// remoteAuthHandler is an AuthHandler that calls a remotely hosted auth handler.
//
// This is used when the auth handler is not being hosted in this container, but in
// another container somewhere else in the cluster.
type remoteAuthHandler struct {
	server         *Server        // The server we're running in
	hostingService config.Service // The service name of the remote auth handler
	authURL        string         // The URL of the remote auth handler
	original       AuthHandler
	logger         zerolog.Logger
	traceLogs      bool
}

var _ AuthHandler = (*remoteAuthHandler)(nil)

func (r *remoteAuthHandler) Authenticate(c IncomingContext) (model.AuthInfo, error) {
	// Quickly check if the auth data is parsable here, if it isn't we can return early
	// without making a remote call.
	if err := r.original.ParseAuthData(c); err != nil {
		return model.AuthInfo{}, err
	}

	if r.traceLogs {
		r.logger.Trace().Msg("calling auth handler")
	}

	// Create the auth request
	authReq, err := http.NewRequestWithContext(c.ctx, http.MethodPost, r.authURL, nil)
	if err != nil {
		r.logger.Err(err).Msg("unable to create auth request")
		return model.AuthInfo{}, errs.Wrap(err, "unable to create auth request")
	}

	// Copy over any data which might be needed by the auth handler (query string, headers and cookies)
	authReq.URL.RawQuery = c.req.URL.RawQuery
	authReq.Header = maps.Clone(c.req.Header)
	delete(authReq.Header, "X-Encore-Auth") // Don't copy the platform auth key across

	// Add call meta data to the request, so the receiving service can use to allow access to the auth handler
	// This also passes a shared TraceID to the auth handler, so it and the service called after the gateway
	// can both be traced as part of the same trace.
	meta := CallMetaFromContext(c.ctx)
	meta.Internal = &InternalCallMeta{
		// Note we're acting as a ApiCaller here so we can access the __encore/auth_handler endpoint and
		// also receive full marshalled errors back from the auth handler (as ApiCaller's are allowed PrivateAPIAccess)
		Caller: &ApiCaller{ServiceName: "gateway", Endpoint: "__encore/authhandler"},
	}
	if err := meta.AddToRequest(r.server, r.hostingService, transport.HTTPRequest(authReq)); err != nil {
		r.logger.Err(err).Msg("unable to add call metadata to auth request")
		return model.AuthInfo{}, errs.Wrap(err, "unable to add call metadata to auth request")
	}

	// Call the auth handler
	resp, err := r.server.httpClient.Do(authReq)
	if err != nil {
		return model.AuthInfo{}, errs.Wrap(err, "unable to make auth request")
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle the response
	switch resp.StatusCode {
	case http.StatusOK:
		// user was authenticated, let's decode it
		t := transport.HTTPResponse(resp)
		uid, found := t.ReadMeta("UserID")
		if !found {
			return model.AuthInfo{}, errs.B().Code(errs.Unauthenticated).Msg("no uid").Err()
		}

		authData := newAuthDataObj()
		if err := authJSON.NewDecoder(resp.Body).Decode(authData); err != nil {
			return model.AuthInfo{}, errs.Wrap(err, "unable to decode auth data")
		}

		return model.AuthInfo{
			UID:      model.UID(uid),
			UserData: authData,
		}, nil

	default:
		// the user was not authenticated, let's return the error
		err := unmarshalErrorResponse(resp)
		if errs.Code(err) != errs.Unauthenticated && r.traceLogs {
			r.logger.Trace().Err(err).Msg("auth handler returned error")
		}

		return model.AuthInfo{}, err
	}
}

// handleRemoteAuthCall is the server side of remoteAuthHandler.Authenticate
func (s *Server) handleRemoteAuthCall(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// Parse the incoming call metadata
	meta := CallMetaFromContext(req.Context())

	// Check if the caller was the gateway, if not return an error
	if meta.Internal == nil || meta.Internal.Caller == nil {
		errs.HTTPErrorWithCode(w, errs.B().Code(errs.PermissionDenied).Msg("permission denied").Err(), 0)
		return
	}
	caller, ok := meta.Internal.Caller.(*ApiCaller)
	if !ok || caller.ServiceName != "gateway" || caller.Endpoint != "__encore/authhandler" {
		errs.HTTPErrorWithCode(w, errs.B().Code(errs.PermissionDenied).Msg("permission denied").Err(), 0)
		return
	}

	// originalC captures the meta _before_ we removed the internal call metadata
	// this is used for returnError to marshal the full error
	originalC := s.NewIncomingContext(w, req, nil, meta)

	// Remove the internal call metadata, so it doesn't get passed to the auth handler
	meta.Internal = nil

	// Create a new IncomingContext
	c := s.NewIncomingContext(w, req, nil, meta)

	// Call the original auth handler
	authInfo, err := s.authHandler.Authenticate(c)
	if err != nil {
		returnError(originalC, err, 0)
		return
	}

	// Add the user ID to the response metadata
	t := transport.HTTPResponseWriter(w)
	t.SetMeta("UserID", string(authInfo.UID))

	// Write the auth info to the response
	if err := authJSON.NewEncoder(w).Encode(authInfo.UserData); err != nil {
		returnError(originalC, err, 0)
		return
	}
}

func (r *remoteAuthHandler) HostedByService() string {
	return r.hostingService.Name
}

func (r *remoteAuthHandler) ParseAuthData(c IncomingContext) error {
	return r.original.ParseAuthData(c)
}
