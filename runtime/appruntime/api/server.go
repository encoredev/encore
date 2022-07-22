package api

import (
	"context"
	"net"
	"net/http"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/cors"
	"encore.dev/appruntime/model"
	"encore.dev/appruntime/platform"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/beta/errs"
	"encore.dev/internal/metrics"
)

type Access string

const (
	Public       Access = "public"
	RequiresAuth Access = "auth"
	Private      Access = "private"
)

type Context struct {
	server *Server
	w      http.ResponseWriter
	req    *http.Request
	ps     httprouter.Params
	auth   model.AuthInfo
}

type Handler interface {
	ServiceName() string
	EndpointName() string
	AccessType() Access
	HTTPPath() string
	HTTPMethods() []string
	Handle(c Context)
}

type Server struct {
	cfg        *config.Config
	rt         *reqtrack.RequestTracker
	pc         *platform.Client // if nil, requests are not authenticated against platform
	rootLogger zerolog.Logger
	json       jsoniter.API

	authHandler AuthHandler

	public  *httprouter.Router
	private *httprouter.Router
	encore  *httprouter.Router
	httpsrv *http.Server

	callCtr uint64

	pubsubSubscriptions map[string]func(r *http.Request) error
}

func NewServer(cfg *config.Config, rt *reqtrack.RequestTracker, pc *platform.Client, rootLogger zerolog.Logger, json jsoniter.API) *Server {
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

	s := &Server{
		cfg:        cfg,
		pc:         pc,
		rt:         rt,
		rootLogger: rootLogger,
		json:       json,

		public:  public,
		private: private,
		encore:  encore,

		pubsubSubscriptions: make(map[string]func(r *http.Request) error),
	}

	// Configure CORS
	corsCfg := &config.CORS{}
	if cfg.Runtime.CORS != nil {
		corsCfg = cfg.Runtime.CORS
	}
	handler := cors.Wrap(corsCfg, http.HandlerFunc(s.handler))
	s.httpsrv = &http.Server{
		Handler: handler,
	}

	s.registerEncoreRoutes()

	return s
}

func (s *Server) Register(handlers []Handler) {
	for _, h := range handlers {
		s.register(h)
	}
}

// SetAuthHandler sets the auth handler to use.
// If h is nil it means no auth handler is used.
func (s *Server) SetAuthHandler(h AuthHandler) {
	s.authHandler = h
}

// wildcardMethod is an internal method name we register wildcard methods under.
const wildcardMethod = "__ENCORE_WILDCARD__"

func (s *Server) register(h Handler) {
	path := h.HTTPPath()
	s.rootLogger.Info().
		Str("service", h.ServiceName()).
		Str("endpoint", h.EndpointName()).
		Str("path", path).
		Msg("registered endpoint")

	for _, m := range h.HTTPMethods() {
		if m == "*" {
			m = wildcardMethod
		}

		adapter := func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			s.processRequest(h, Context{
				server: s,
				w:      w,
				req:    req,
				ps:     ps,
			})
		}

		s.private.Handle(m, path, adapter)
		if access := h.AccessType(); access == Public || access == RequiresAuth {
			s.public.Handle(m, path, adapter)
		}
	}
}

func (s *Server) Serve(ln net.Listener) error {
	s.rootLogger.Info().Msg("listening for incoming HTTP requests")
	return s.httpsrv.Serve(ln)
}

func (s *Server) Shutdown(force context.Context) {
	_ = s.httpsrv.Shutdown(force)
}

func (s *Server) handler(w http.ResponseWriter, req *http.Request) {
	ep := strings.TrimPrefix(req.URL.Path, "/")

	// Select a router based on access
	r := s.public

	// The Encore platform is authorised to call private APIs directly, thus if we have this header set,
	// and authenticate it, then we can switch over to the private router which contains all APIs not just
	// the publicly accessible ones.
	if sig := req.Header.Get("X-Encore-Auth"); sig != "" && s.pc != nil {
		if ok, err := s.pc.ValidatePlatformRequest(req, sig); err == nil && ok {
			// Successfully authenticated
			req = req.WithContext(withEncorePlatformSealOfApproval(req.Context()))
			r = s.private
		} else if err != nil {
			http.Error(w, "could not authenticate request", http.StatusBadGateway)
			return
		} else {
			http.Error(w, "invalid request signature", http.StatusUnauthorized)
			return
		}
	}

	// We use EscapedPath rather than `req.URL.Path` because if the path contains an encoded
	// forward slash as %2F we don't want the router to treat that as a segment split.
	//
	// i.e. `/foo%2Fbar/baz` should be routed to `/:a/*b` as a = "foo/bar", b = "baz"
	// where as if we use req.URL.Path we would get a = "foo", b = "bar/baz` which is incorrect.
	path := req.URL.EscapedPath()

	// Switch to the Encore internal router if we are on the Encore internal path
	const internalPrefix = "/__encore"
	if strings.HasPrefix(path, internalPrefix+"/") {
		r = s.encore
		path = path[len(internalPrefix):] // keep leading slash
	}

	h, p, _ := r.Lookup(req.Method, path)
	if h == nil {
		h, p, _ = r.Lookup(wildcardMethod, path)
	}
	if h != nil {
		// Found an endpoint
		h(w, req, p)
		return
	}

	// Endpoint not found
	svc, api := "unknown", "Unknown"
	if idx := strings.IndexByte(ep, '.'); idx != -1 {
		svc, api = ep[:idx], ep[idx+1:]
	}
	metrics.UnknownEndpoint(svc, api)
	errs.HTTPError(w, errs.B().Code(errs.NotFound).Msg("endpoint not found").Err())
}

func (s *Server) processRequest(h Handler, c Context) {
	c.server.beginOperation()
	defer c.server.finishOperation()

	info, proceed := s.runAuthHandler(h, c)
	if proceed {
		c.auth = info
		h.Handle(c)
	}
}

func (s *Server) NewContext(w http.ResponseWriter, req *http.Request, ps PathParams, auth model.AuthInfo) Context {
	return Context{s, w, req, ps, auth}
}

func (s *Server) NewCallContext(ctx context.Context) CallContext {
	return CallContext{ctx, s}
}

type encoreAuthenticateCtxKey string

const encoreAuthenticatedKey encoreAuthenticateCtxKey = "encoreAuthenticateCtxKey"

func withEncorePlatformSealOfApproval(ctx context.Context) context.Context {
	return context.WithValue(ctx, encoreAuthenticatedKey, true)
}

// IsEncorePlatformRequest returns true if the given context originated from
// a request from the Encore Platform.
func IsEncorePlatformRequest(ctx context.Context) bool {
	value := ctx.Value(encoreAuthenticatedKey)
	if value == nil {
		return false
	}

	v, ok := value.(bool)
	return ok && v
}
