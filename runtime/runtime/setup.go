package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"

	"encore.dev/beta/errs"
	"encore.dev/internal/ctx"
	"encore.dev/internal/metrics"
	"encore.dev/runtime/config"
	"encore.dev/runtime/cors"
)

var defaultServer = setup()

func ListenAndServe() error {
	return defaultServer.ListenAndServe()
}

type Server struct {
	logger  *zerolog.Logger
	public  *httprouter.Router
	private *httprouter.Router
	encore  *httprouter.Router // Internal Encore routes
}

// wildcardMethod is an internal method name we register wildcard methods under.
const wildcardMethod = "__ENCORE_WILDCARD__"

func (srv *Server) handleRPC(service string, endpoint *config.Endpoint) {
	srv.logger.Info().Str("service", service).Str("endpoint", endpoint.Name).Str("path", endpoint.Path).Msg("registered endpoint")
	for _, m := range endpoint.Methods {
		if m == "*" {
			m = wildcardMethod
		}
		srv.private.Handle(m, endpoint.Path, endpoint.Handler)
		if endpoint.Access == config.Public || endpoint.Access == config.Auth {
			srv.public.Handle(m, endpoint.Path, endpoint.Handler)
		}
	}
}

func (srv *Server) ListenAndServe() error {
	ln, err := listen()
	if err != nil {
		return err
	}

	// Configure CORS
	corsCfg := &config.CORS{}
	if cfg := config.Cfg; cfg != nil && cfg.Runtime.CORS != nil {
		corsCfg = cfg.Runtime.CORS
	}
	handler := cors.Wrap(corsCfg, http.HandlerFunc(srv.handler))

	httpsrv := &http.Server{
		Handler: handler,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx.App
		},
	}
	srv.logger.Info().Msg("listening for incoming HTTP requests")

	RegisterShutdown(func(force context.Context) {
		httpsrv.Shutdown(force)
	})
	serveErr := httpsrv.Serve(ln)

	var shutdownResult error
	select {
	case <-shutdown.initiated:
		// This is a graceful shutdown; wait for the shutdown to complete before returning.
		shutdownResult = nil
	default:
		// This is not due to a shutdown signal; return the error from Serve
		doShutdown()
		shutdownResult = serveErr
	}

	// Wait for shutdown to complete before returning, since returning causes the process to exit.
	<-shutdown.completed
	return shutdownResult
}

func (srv *Server) handler(w http.ResponseWriter, req *http.Request) {
	// We use EscapedPath rather than `req.URL.Path` because if the path contains an encoded
	// forward slash as %2F we don't want the router to treat that as a segment split.
	//
	// i.e. `/foo%2Fbar/baz` should be routed to `/:a/*b` as a = "foo/bar", b = "baz"
	// where as if we use req.URL.Path we would get a = "foo", b = "bar/baz` which is incorrect.
	path := req.URL.EscapedPath()

	// Select a router based on access
	r := srv.public

	// The Encore platform is authorised to call private APIs directly, thus if we have this header set,
	// and authenticate it, then we can switch over to the private router which contains all API's not just
	// the publicly accessible ones.
	if h := req.Header.Get("X-Encore-Auth"); h != "" {
		if ok, err := srv.checkAuth(req, h); err == nil && ok {
			// Successfully authenticated
			req = req.WithContext(withEncoreAuthentication(req.Context()))
			r = srv.private
		} else if err != nil {
			errs.HTTPError(w, errs.B().Code(errs.Internal).Msg("could not authenticate request").Err())
			return
		} else {
			errs.HTTPError(w, errs.B().Code(errs.Unauthenticated).Msg("invalid request signature").Err())
			return
		}
	}

	// Switch to the Encore internal router if we are on a Encore internal path
	// Historically we used a prefix of `/__encore` but this is now deprecated in favour of a well-known `encore` path
	// But for now we support both
	if strings.HasPrefix(path, "/__encore/") || strings.HasPrefix(path, "/.well-known/encore/") {
		r = srv.encore
		path = strings.TrimPrefix(strings.TrimPrefix(path, "/__encore"), "/.well-known/encore")
	}

	h, p, _ := r.Lookup(req.Method, path)
	if h == nil {
		h, p, _ = r.Lookup(wildcardMethod, path)
	}
	if h == nil {
		svc, api := "unknown", "Unknown"

		ep := strings.TrimPrefix(path, "/")
		if idx := strings.IndexByte(ep, '.'); idx != -1 {
			svc, api = ep[:idx], ep[idx+1:]
		}
		metrics.UnknownEndpoint(svc, api)
		errs.HTTPError(w, errs.B().Code(errs.NotFound).Msg("endpoint not found").Err())
		return
	}

	h(w, req, p)
}

func (srv *Server) scrapeMetrics(w http.ResponseWriter, req *http.Request) {
	mfs, err := metrics.Gather()
	if err != nil {
		http.Error(w, "could not gather metrics: "+err.Error(), http.StatusInternalServerError)
		return
	}
	enc := expfmt.NewEncoder(w, expfmt.FmtProtoDelim)
	for _, mf := range mfs {
		if err := enc.Encode(mf); err != nil {
			http.Error(w, "could not encode metrics: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func setup() *Server {
	public := httprouter.New()
	public.HandleOPTIONS = false
	public.RedirectFixedPath = false
	public.RedirectTrailingSlash = false

	private := httprouter.New()
	private.HandleOPTIONS = false
	private.RedirectFixedPath = false
	private.RedirectTrailingSlash = false

	encore := httprouter.New()
	encore.HandleOPTIONS = false
	encore.RedirectFixedPath = false
	encore.RedirectTrailingSlash = false
	registerEncoreRoutes(encore)

	srv := &Server{
		logger:  Logger(),
		public:  public,
		private: private,
		encore:  encore,
	}

	if config.Cfg != nil {
		for _, svc := range config.Cfg.Static.Services {
			for _, endpoint := range svc.Endpoints {
				srv.handleRPC(svc.Name, endpoint)
			}
		}
	}

	return srv
}

func checkAuth(key config.EncoreAuthKey, req *http.Request, gotMac []byte) bool {
	dateStr := req.Header.Get("Date")
	if dateStr == "" {
		return false
	}
	date, err := http.ParseTime(dateStr)
	if err != nil {
		return false
	}
	const threshold = 15 * time.Minute
	if diff := time.Since(date); diff > threshold || diff < -threshold {
		return false
	}

	mac := hmac.New(sha256.New, key.Data)
	fmt.Fprintf(mac, "%s\x00%s", dateStr, req.URL.Path)
	expected := mac.Sum(nil)
	return hmac.Equal(expected, gotMac)
}
