package api

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/benbjohnson/clock"
	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/config"
	"encore.dev/appruntime/cors"
	"encore.dev/appruntime/model"
	"encore.dev/appruntime/platform"
	"encore.dev/appruntime/reqtrack"
	"encore.dev/beta/errs"
	"encore.dev/metrics"
)

type Access string

const (
	Public       Access = "public"
	RequiresAuth Access = "auth"
	Private      Access = "private"
)

// execContext contains the data needed for executing a request.
type execContext struct {
	server  *Server
	ctx     context.Context
	ps      UnnamedParams
	traceID model.TraceID
	auth    model.AuthInfo
}

type IncomingContext struct {
	execContext
	w   http.ResponseWriter
	req *http.Request

	// capturer is set in handleIncoming for raw requests
	// to capture the request body
	capturer *rawRequestBodyCapturer
}

type Handler interface {
	ServiceName() string
	EndpointName() string
	AccessType() Access
	SemanticPath() string
	HTTPRouterPath() string
	HTTPMethods() []string
	SetMiddleware([]*Middleware)
	Handle(c IncomingContext)
}

type requestsTotalLabels struct {
	endpoint string // Endpoint name.
	code     string // Human-readable HTTP status code.
}

type Server struct {
	cfg            *config.Config
	rt             *reqtrack.RequestTracker
	pc             *platform.Client // if nil, requests are not authenticated against platform
	encoreMgr      *encore.Manager
	requestsTotal  *metrics.CounterGroup[requestsTotalLabels, uint64]
	clock          clock.Clock
	rootLogger     zerolog.Logger
	json           jsoniter.API
	tracingEnabled bool

	authHandler AuthHandler

	public  *httprouter.Router
	private *httprouter.Router
	encore  *httprouter.Router
	httpsrv *http.Server

	callCtr uint64

	pubsubSubscriptions map[string]func(r *http.Request) error
}

func NewServer(
	cfg *config.Config,
	rt *reqtrack.RequestTracker,
	pc *platform.Client,
	encoreMgr *encore.Manager,
	rootLogger zerolog.Logger,
	reg *metrics.Registry,
	json jsoniter.API,
	tracingEnabled bool,
	clock clock.Clock,
) *Server {
	requestsTotal := metrics.NewCounterGroupInternal[requestsTotalLabels, uint64](reg, "e_requests_total", metrics.CounterConfig{
		EncoreInternal_LabelMapper: func(labels requestsTotalLabels) []metrics.KeyValue {
			return []metrics.KeyValue{
				{Key: "endpoint", Value: labels.endpoint},
				{Key: "code", Value: labels.code},
			}
		},
	})

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
		cfg:            cfg,
		pc:             pc,
		rt:             rt,
		encoreMgr:      encoreMgr,
		requestsTotal:  requestsTotal,
		clock:          clock,
		rootLogger:     rootLogger,
		json:           json,
		tracingEnabled: tracingEnabled,

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
	handler := cors.Wrap(
		corsCfg,
		cfg.Static.CORSAllowHeaders,
		cfg.Static.CORSExposeHeaders,
		http.HandlerFunc(s.handler),
	)
	s.httpsrv = &http.Server{
		Handler: handler,
	}

	s.registerEncoreRoutes()

	return s
}

// SetAuthHandler sets the auth handler to use.
// If h is nil it means no auth handler is used.
func (s *Server) SetAuthHandler(h AuthHandler) {
	s.authHandler = h
}

type HandlerRegistration struct {
	Handler    Handler
	Middleware []*Middleware
}

func (s *Server) Register(handlers []HandlerRegistration) {
	// If we have a lot of handlers, don't log each one being registered.
	logEachRegistration := len(handlers) < 8 // chosen by a fair dice roll

	// Don't log during tests.
	if s.cfg.Static.Testing {
		logEachRegistration = false
	}

	for _, h := range handlers {
		s.register(h, logEachRegistration)
	}

	if !logEachRegistration && !s.cfg.Static.Testing {
		s.rootLogger.Info().Msgf("registered %d API endpoints", len(handlers))
	}
}

// wildcardMethod is an internal method name we register wildcard methods under.
const wildcardMethod = "__ENCORE_WILDCARD__"

func (s *Server) register(reg HandlerRegistration, logRegistration bool) {
	h := reg.Handler
	h.SetMiddleware(reg.Middleware)

	if logRegistration {
		s.rootLogger.Info().
			Str("service", h.ServiceName()).
			Str("endpoint", h.EndpointName()).
			Str("path", h.SemanticPath()).
			Msg("registered API endpoint")
	}

	for _, m := range h.HTTPMethods() {
		if m == "*" {
			m = wildcardMethod
		}

		adapter := func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			params := toUnnamedParams(ps)
			traceID, _ := model.GenTraceID()
			traceIDStr := traceID.String()

			// Echo the X-Request-ID back to the caller if present,
			// otherwise send back the trace id.
			reqID := req.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = traceIDStr
			} else if len(reqID) > 64 {
				// Don't allow arbitrarily long request IDs.
				s.rootLogger.Warn().Int("length", len(reqID)).Msg("X-Request-ID was too long and is being truncated to 64 characters")
				reqID = reqID[:64]
			}
			w.Header().Set("X-Request-ID", reqID)

			// Read the correlation ID from the request.
			correlationID := req.Header.Get("X-Correlation-ID")
			if len(correlationID) > 64 {
				// Don't allow arbitrarily long correlation IDs.
				s.rootLogger.Warn().Int("length", len(reqID)).Msg("X-Correlation-ID was too long and is being truncated to 64 characters")
				correlationID = correlationID[:64]
			}
			if correlationID != "" {
				w.Header().Set("X-Correlation-ID", correlationID)
			}

			// Always send the trace id back.
			w.Header().Set("X-Encore-Trace-ID", traceIDStr)

			s.processRequest(h, s.NewIncomingContext(w, req, params, traceID, model.AuthInfo{}))
		}

		routerPath := h.HTTPRouterPath()
		s.private.Handle(m, routerPath, adapter)
		if access := h.AccessType(); access == Public || access == RequiresAuth {
			s.public.Handle(m, routerPath, adapter)
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
	// Select a router based on access
	r := s.public

	// The Encore platform is authorised to call private APIs directly, thus if we have this header set,
	// and authenticate it, then we can switch over to the private router which contains all APIs not just
	// the publicly accessible ones.
	if sig := req.Header.Get("X-Encore-Auth"); sig != "" && s.pc != nil {
		// Delete the header so it can't be accessed.
		req.Header.Del("X-Encore-Auth")

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

	if h == nil {
		// If we still couldn't find a handler, check if there is one
		// with a trailing slash redirect.
		if handleTrailingSlashRedirect(r, w, req, path) {
			return
		}
	}

	if h != nil {
		// Found an endpoint
		h(w, req, p)
		return
	}

	// Endpoint not found
	errs.HTTPError(w, errs.B().Code(errs.NotFound).Msg("endpoint not found").Err())
}

func (s *Server) processRequest(h Handler, c IncomingContext) {
	c.server.beginOperation()
	defer c.server.finishOperation()

	info, proceed := s.runAuthHandler(h, c)
	if proceed {
		c.auth = info
		h.Handle(c)
	}
}

func (s *Server) newExecContext(ctx context.Context, ps UnnamedParams, trID model.TraceID, auth model.AuthInfo) execContext {
	return execContext{s, ctx, ps, trID, auth}
}

func (s *Server) NewIncomingContext(w http.ResponseWriter, req *http.Request, ps UnnamedParams, trID model.TraceID, auth model.AuthInfo) IncomingContext {
	ec := s.newExecContext(req.Context(), ps, trID, auth)
	return IncomingContext{ec, w, req, nil}
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

var Singleton *Server // for use in generated code

func NewCallContext(ctx context.Context) CallContext {
	return Singleton.NewCallContext(ctx)
}

func toUnnamedParams(ps httprouter.Params) UnnamedParams {
	params := make(UnnamedParams, len(ps))
	for i, p := range ps {
		params[i] = p.Value
	}
	return params
}

// handleTrailingSlashRedirect checks if there's a matching handler
// with (without) a trailing slash and redirects to it if there is.
//
// This is a modified version of the built-in support in httprouter.
// We can't use the built-in one due to how we handle multiple methods
// for the same route using Lookup instead of Handle.
func handleTrailingSlashRedirect(r *httprouter.Router, w http.ResponseWriter, req *http.Request, path string) (handled bool) {
	// CONNECT does not support redirects.
	if req.Method == http.MethodConnect {
		return false
	}

	// Modify the path to include (exclude) the trailing slash.
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	} else {
		path += "/"
	}

	// Check if there is a handler for the modified path.
	h, _, _ := r.Lookup(req.Method, path)
	if h == nil {
		h, _, _ = r.Lookup(wildcardMethod, path)
	}

	if h == nil {
		// Couldn't find a handler.
		return false
	}

	// Moved Permanently, request with GET method
	code := http.StatusMovedPermanently
	if req.Method != http.MethodGet {
		// Permanent Redirect, request with same method
		code = http.StatusPermanentRedirect
	}

	http.Redirect(w, req, path, code)
	return true
}
