package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/benbjohnson/clock"
	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	encore "encore.dev"
	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/apisdk/cors"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/shared/platform"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/beta/errs"
	"encore.dev/internal/platformauth"
	"encore.dev/metrics"
	"encore.dev/pubsub"
)

type Access string

const (
	Public       Access = "public"
	RequiresAuth Access = "auth"
	Private      Access = "private"
)

const (
	// eventTraceStateKey is the key used to store the event ID in the trace state passed between instances
	// It is encoded as a base36 string of the underlying uint64.
	eventTraceStateKey = "encore/event-id"
)

// execContext contains the data needed for executing a request.
type execContext struct {
	server *Server
	ctx    context.Context
	ps     UnnamedParams
	auth   model.AuthInfo

	traceID       model.TraceID      // zero if not tracing
	parentSpanID  model.SpanID       // zero if no parent
	parentEventID model.TraceEventID // zero if no parent
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
	IsFallback() bool
	Handle(c IncomingContext)
}

type requestsTotalLabels struct {
	endpoint string // Endpoint name.
	code     string // Human-readable HTTP status code.
}

type Server struct {
	static         *config.Static
	runtime        *config.Runtime
	rt             *reqtrack.RequestTracker
	pc             *platform.Client // if nil, requests are not authenticated against platform
	encoreMgr      *encore.Manager
	pubsubMgr      *pubsub.Manager
	requestsTotal  *metrics.CounterGroup[requestsTotalLabels, uint64]
	httpClient     *http.Client
	clock          clock.Clock
	rootLogger     zerolog.Logger
	json           jsoniter.API
	tracingEnabled bool

	authHandler AuthHandler

	globalMiddleware   map[string]*Middleware
	registeredHandlers []Handler

	public          *httprouter.Router
	publicFallback  *httprouter.Router
	private         *httprouter.Router
	privateFallback *httprouter.Router
	encore          *httprouter.Router
	httpsrv         *http.Server

	callCtr uint64

	pubsubSubscriptions map[string]func(r *http.Request) error
}

func NewServer(
	static *config.Static,
	runtime *config.Runtime,
	rt *reqtrack.RequestTracker,
	pc *platform.Client,
	encoreMgr *encore.Manager,
	pubsubMgr *pubsub.Manager,
	rootLogger zerolog.Logger,
	reg *metrics.Registry,
	json jsoniter.API,
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

	newRouter := func() *httprouter.Router {
		router := httprouter.New()
		router.HandleOPTIONS = false
		router.RedirectFixedPath = false
		router.RedirectTrailingSlash = false
		return router
	}

	s := &Server{
		static:         static,
		runtime:        runtime,
		pc:             pc,
		rt:             rt,
		encoreMgr:      encoreMgr,
		pubsubMgr:      pubsubMgr,
		requestsTotal:  requestsTotal,
		httpClient:     &http.Client{},
		clock:          clock,
		rootLogger:     rootLogger,
		json:           json,
		tracingEnabled: rt.TracingEnabled(),

		public:          newRouter(),
		publicFallback:  newRouter(),
		private:         newRouter(),
		privateFallback: newRouter(),
		encore:          newRouter(),
	}

	// Configure CORS
	corsCfg := &config.CORS{}
	if runtime.CORS != nil {
		corsCfg = runtime.CORS
	}
	handler := cors.Wrap(
		corsCfg,
		static.CORSAllowHeaders,
		static.CORSExposeHeaders,
		http.HandlerFunc(s.handler),
	)
	s.httpsrv = &http.Server{
		Handler: handler,
	}

	s.registerEncoreRoutes()

	return s
}

// setAuthHandler sets the auth handler to use.
// If h is nil it means no auth handler is used.
func (s *Server) setAuthHandler(h AuthHandler) {
	s.authHandler = h
}

func (s *Server) RegisteredHandlers() []Handler {
	return s.registeredHandlers
}

// wildcardMethod is an internal method name we register wildcard methods under.
const wildcardMethod = "__ENCORE_WILDCARD__"

func (s *Server) registerEndpoint(h Handler) {
	s.registeredHandlers = append(s.registeredHandlers, h)

	for _, m := range h.HTTPMethods() {
		if m == "*" {
			m = wildcardMethod
		}

		adapter := func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
			params := toUnnamedParams(ps)

			// Extract metadata from the request.
			meta, err := MetaFromRequest(transport.HTTPRequest(req))
			if err != nil {
				s.rootLogger.Error().Err(err).Msg("failed to extract metadata from request")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if meta.TraceID.IsZero() {
				meta.TraceID, _ = model.GenTraceID()
				meta.ParentSpanID = model.SpanID{} // no parent span if we have no trace id
			}
			traceIDStr := meta.TraceID.String()

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
			if meta.CorrelationID != "" {
				w.Header().Set("X-Correlation-ID", meta.CorrelationID)
			}

			// Always send the trace id back.
			w.Header().Set("X-Encore-Trace-ID", traceIDStr)

			s.processRequest(h, s.NewIncomingContext(w, req, params, meta))
		}

		routerPath := h.HTTPRouterPath()

		// Decide which routers to use.
		private, public := s.private, s.public
		if h.IsFallback() {
			private, public = s.privateFallback, s.publicFallback
		}

		private.Handle(m, routerPath, adapter)
		if access := h.AccessType(); access == Public || access == RequiresAuth {
			public.Handle(m, routerPath, adapter)
		}
	}
}

func (s *Server) registerGlobalMiddleware(mw *Middleware) {
	if s.globalMiddleware == nil {
		s.globalMiddleware = make(map[string]*Middleware)
	}
	s.globalMiddleware[mw.ID] = mw
}

func (s *Server) getGlobalMiddleware(ids []string) []*Middleware {
	// Don't add global middleware when tests are executing,
	// as it's not possible to guarantee all global middleware
	// have actually been imported when the tests run.
	if s.static.Testing {
		return nil
	}

	result := make([]*Middleware, 0, len(ids))
	for _, id := range ids {
		mw, ok := s.globalMiddleware[id]
		if !ok {
			panic(fmt.Sprintf("middleware %q not registered", id))
		}
		result = append(result, mw)
	}
	return result
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
	router, fallbackRouter := s.public, s.publicFallback

	// The Encore platform is authorised to call private APIs directly, thus if we have this header set,
	// and authenticate it, then we can switch over to the private router which contains all APIs not just
	// the publicly accessible ones.
	if sig := req.Header.Get("X-Encore-Auth"); sig != "" && s.pc != nil {
		// Delete the header so it can't be accessed.
		req.Header.Del("X-Encore-Auth")

		if ok, err := s.pc.ValidatePlatformRequest(req, sig); err == nil && ok {
			// Successfully authenticated
			req = req.WithContext(platformauth.WithEncorePlatformSealOfApproval(req.Context()))
			router, fallbackRouter = s.private, s.privateFallback
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
		router, fallbackRouter = s.encore, nil
		path = path[len(internalPrefix):] // keep leading slash
	}

	findRoute := func(r *httprouter.Router) (h httprouter.Handle, p httprouter.Params, handledTSR bool) {
		h, p, _ = r.Lookup(req.Method, path)
		if h == nil {
			h, p, _ = r.Lookup(wildcardMethod, path)
		}

		if h == nil {
			// If we still couldn't find a handler, check if there is one
			// with a trailing slash redirect.
			if handleTrailingSlashRedirect(r, w, req, path) {
				return nil, nil, true
			}
		}

		return h, p, false
	}

	// Find the route. Try first with the chosen router and otherwise check the fallback router.
	h, p, handled := findRoute(router)
	if !handled && h == nil && fallbackRouter != nil {
		h, p, handled = findRoute(fallbackRouter)
	}

	// If the router already handled the request via a trailing-slash redirect, we're done.
	if handled {
		return
	}

	if h != nil {
		// Found an endpoint.
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

func (s *Server) newExecContext(ctx context.Context, ps UnnamedParams, callMeta CallMeta) execContext {
	var auth model.AuthInfo
	if callMeta.Internal != nil {
		auth = model.AuthInfo{
			UID:      model.UID(callMeta.Internal.AuthUID),
			UserData: callMeta.Internal.AuthData,
		}
	}
	return execContext{s, ctx, ps, auth, callMeta.TraceID, callMeta.ParentSpanID, callMeta.ParentEventID}
}

func (s *Server) NewIncomingContext(w http.ResponseWriter, req *http.Request, ps UnnamedParams, callMeta CallMeta) IncomingContext {
	ec := s.newExecContext(req.Context(), ps, callMeta)
	return IncomingContext{ec, w, req, nil}
}

func (s *Server) NewCallContext(ctx context.Context) CallContext {
	return CallContext{ctx, s}
}

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
