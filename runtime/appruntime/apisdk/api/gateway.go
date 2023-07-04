package api

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/beta/errs"
)

// IsGateway returns true if this instance of the container is acting as an API
// gateway
func (s *Server) IsGateway() bool {
	return len(s.runtime.Gateways) > 0
}

// createGatewayHandlerAdapter creates a httprouter.Handle that proxies requests
// ontop of the given handler to the service that is hosting the handler.
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
	logger := s.rootLogger.With().Str("service", service.Name).Str("endpoint", h.EndpointName()).Logger()

	proxy := httputil.NewSingleHostReverseProxy(serviceBaseURL)
	proxy.ErrorLog = newZeroLogAdapter(logger, zerolog.ErrorLevel)
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		logger.Err(err).Msg("error proxying request to service")
		errs.HTTPError(w, errs.B().Cause(err).Code(errs.Unavailable).Err())
	}

	return func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		if s.runtime.EnvCloud != "local" {
			logger.Trace().Msg("proxying request to service")
		}
		proxy.ServeHTTP(w, req)
	}
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
