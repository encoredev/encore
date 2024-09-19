package api

import (
	"io"
	"maps"
	"net/http"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
)

// remotePubSubPushHandler is a handler that calls a remotely hosted push subscription handler.
//
// This is used when the push subscription handler is not being hosted in this container, but in
// another container somewhere else in the cluster.
type remotePubSubPushHandler struct {
	server         *Server        // The server we're running in
	hostingService config.Service // The service name of the remote subscription handler
	pushURL        string         // The URL of the remote subscription handler
	logger         zerolog.Logger
}

func (r *remotePubSubPushHandler) ForwardRequest(w http.ResponseWriter, req *http.Request) error {
	// Create the remote push request
	pushReq, err := http.NewRequestWithContext(req.Context(), req.Method, r.pushURL, req.Body)
	if err != nil {
		r.logger.Err(err).Msg("unable to create remote push request")
		return errs.Wrap(err, "unable to create remote push request")
	}

	// Copy over any data which might be needed by the subscription handler (query string, headers and cookies)
	pushReq.URL.RawQuery = req.URL.RawQuery
	pushReq.Header = maps.Clone(req.Header)

	// Add call meta data to the request
	// This also passes a shared TraceID to the subscription handler, so it and the service called after the gateway
	// can both be traced as part of the same trace.
	meta := CallMetaFromContext(req.Context())
	meta.Internal = &InternalCallMeta{
		Caller: ApiCaller{ServiceName: "gateway", Endpoint: req.URL.Path},
	}
	if err := meta.AddToRequest(r.server, r.hostingService, transport.HTTPRequest(pushReq)); err != nil {
		r.logger.Err(err).Msg("unable to add call metadata to remote push request")
		return errs.Wrap(err, "unable to add call metadata to remote push request")
	}

	// Call the sub handler
	resp, err := r.server.httpClient.Do(pushReq)
	if err != nil {
		r.logger.Err(err).Msg("unable to make remote push request")
		return errs.Wrap(err, "unable to make remote push request")
	}
	defer func() { _ = resp.Body.Close() }()

	// Copy the headers from the proxy response to the original response
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		r.logger.Err(err).Msg("unable to copy remote push response")
		return errs.Wrap(err, "unable to copy remote push response")
	}
	return nil
}

func (r *remotePubSubPushHandler) HostedByService() string {
	return r.hostingService.Name
}
