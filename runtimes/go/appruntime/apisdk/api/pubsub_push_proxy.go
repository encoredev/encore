package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
)

func (s *Server) createPubsubPushProxy(target config.Service) (*httputil.ReverseProxy, error) {
	targetUrl, err := url.Parse(target.URL)

	if err != nil {
		return nil, err
	}
	logger := s.rootLogger.With().Str("remote_push_url", target.URL).Logger()

	return &httputil.ReverseProxy{
		// Rewrite the inbound request
		Rewrite: func(req *httputil.ProxyRequest) {
			req.SetURL(targetUrl)
			t := transport.HTTPRequest(req.Out)
			meta := CallMetaFromContext(req.In.Context())
			if err := meta.AddToRequest(s, target, t); err != nil {
				logger.Err(err).Msg("failed to add call metadata to request")
			}
		},
		// Have the reverse proxy log errors to our logger.
		ErrorLog: newZeroLogAdapter(logger, zerolog.ErrorLevel),
		// Handle proxy errors using our error handler output
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			logger.Err(err).Msg("error proxying request to service")
			errs.HTTPError(w, errs.B().Cause(err).Code(errs.Unavailable).Err())
		},
	}, nil
}
