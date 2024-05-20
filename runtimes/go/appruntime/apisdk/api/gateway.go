package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"

	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
)

// IsGateway returns true if this instance of the container is acting as an API
// gateway
func (s *Server) IsGateway() bool {
	return len(s.runtime.Gateways) > 0
}

// createGatewayHandlerAdapter creates a httprouter.Handle that proxies requests
// on top of the given handler to the service that is hosting the handler.
func (s *Server) createGatewayHandlerAdapter(h Handler) httprouter.Handle {
	service, found := s.runtime.ServiceDiscovery[h.ServiceName()]
	if !found {
		panic(fmt.Sprintf("service %q not found in service discovery when hosted in gateway", h.ServiceName()))
	}

	serviceBaseURL, err := url.Parse(service.URL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse service URL %q: %v", service.URL, err))
	}

	// On cloud environments, we want to log the proxying of requests to services
	// but locally we don't want the overhead of logging every request.
	logger := s.rootLogger.With().Str("service", service.Name).Str("endpoint", h.EndpointName()).Str("base_url", serviceBaseURL.String()).Logger()

	proxy := s.createProxyToService(service, h.EndpointName(), serviceBaseURL, logger)
	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		s.beginOperation()
		defer s.finishOperation()

		meta := CallMetaFromContext(req.Context())

		info, proceed := s.runAuthHandler(h, s.NewIncomingContext(w, req, toUnnamedParams(ps), meta))
		if proceed {
			meta.Internal = &InternalCallMeta{
				Caller: GatewayCaller{
					GatewayName: "api-gateway",
				},
				AuthUID:  string(info.UID),
				AuthData: info.UserData,
			}
			req = req.WithContext(SetCallMetaInContext(req.Context(), meta))

			if s.runtime.EnvCloud != "local" {
				logger.Trace().Msg("proxying request to service")
			}
			proxy.ServeHTTP(w, req)
		}
	}
}

// createProxyToService creates a httputil.ReverseProxy that proxies requests onto the target service.
func (s *Server) createProxyToService(service config.Service, endpointName string, serviceBaseURL *url.URL, logger zerolog.Logger) *httputil.ReverseProxy {
	callee := fmt.Sprintf("%s.%s", service.Name, endpointName)

	proxy := &httputil.ReverseProxy{
		// Rewrite the inbound request
		Rewrite: func(req *httputil.ProxyRequest) {
			req.SetURL(serviceBaseURL)

			t := transport.HTTPRequest(req.Out)
			t.SetMeta(calleeMetaName, callee) // required by the Handler which verifies we wanted to call this endpoint

			meta := CallMetaFromContext(req.In.Context())
			if err := meta.AddToRequest(s, service, t); err != nil {
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
	}

	// If the service is served without TLS, we need to configure the proxy to allow forwarding
	// HTTP2 in clear text to make sure grpc requests are forwarded correctly.
	if serviceBaseURL.Scheme == "http" {
		proxy.Transport = &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		}
	}
	return proxy
}

type zeroLogWriter struct {
	logger zerolog.Logger
	level  zerolog.Level
}

func (z *zeroLogWriter) Write(p []byte) (n int, err error) {
	z.logger.WithLevel(z.level).CallerSkipFrame(3).Msg(string(p))
	return len(p), nil
}

// NewZeroLogAdapter returns a new log.Logger that writes to the given zerolog.Logger at the given level.
func newZeroLogAdapter(logger zerolog.Logger, level zerolog.Level) *log.Logger {
	zlw := &zeroLogWriter{logger, level}
	return log.New(zlw, "", 0)
}
