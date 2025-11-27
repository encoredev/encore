package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/benbjohnson/clock"
	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	encore "encore.dev"
	"encore.dev/appruntime/apisdk/api/svcauth"
	"encore.dev/appruntime/apisdk/api/transport"
	"encore.dev/appruntime/apisdk/cors"
	"encore.dev/appruntime/exported/config"
	"encore.dev/appruntime/exported/experiments"
	"encore.dev/appruntime/exported/model"
	"encore.dev/appruntime/shared/cfgutil"
	"encore.dev/appruntime/shared/cloudtrace"
	"encore.dev/appruntime/shared/health"
	"encore.dev/appruntime/shared/platform"
	"encore.dev/appruntime/shared/reqtrack"
	"encore.dev/appruntime/shared/shutdown"
	"encore.dev/appruntime/shared/testsupport"
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
	// eventTraceStateEventIDKey is the key used to store the event ID in the trace state passed between instances
	// It is encoded as a base36 string of the underlying uint64.
	eventTraceStateEventIDKey = "encore/event-id"

	// eventTraceStateSpanIDKey is the key used to store the span ID in the trace state passed between instances
	// We require this on GCP because the traceparent gets a new span ID inserted by GCP's own Trace implementation
	// (I suspect for the load balancers) and so the span ID in the traceparent is not the same as the span ID
	// need to create a child span.
	eventTraceStateSpanIDKey = "encore/span-id"
)

// execContext contains the data needed for executing a request.
type execContext struct {
	server *Server
	ctx    context.Context
	ps     UnnamedParams
	auth   model.AuthInfo

	callMeta CallMeta
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
	experiments    *experiments.Set // The set of experiments enabled for this runtime

	authHandler AuthHandler

	globalMiddleware    map[string]*Middleware
	registeredHandlers  []Handler
	functionsToHandlers map[uintptr]Handler

	public           *httprouter.Router
	publicFallback   *httprouter.Router
	private          *httprouter.Router
	privateFallback  *httprouter.Router
	encore           *httprouter.Router
	inboundSvcAuth   map[string]svcauth.ServiceAuth // auth methods used to accept inbound service-to-service calls
	outboundSvcAuth  map[string]svcauth.ServiceAuth // auth methods used to make outbound service-to-service calls
	httpsrv          *http.Server
	httpCtx          context.Context
	httpCtxCancel    context.CancelFunc
	runningHandlers  sync.WaitGroup
	remotePubSubPush map[string]*httputil.ReverseProxy

	callCtr uint64

	pubsubSubscriptions map[string]func(r *http.Request) error
	healthMgr           *health.CheckRegistry
	testingMgr          *testsupport.Manager
}

func NewServer(static *config.Static, runtime *config.Runtime, rt *reqtrack.RequestTracker, pc *platform.Client, encoreMgr *encore.Manager, pubsubMgr *pubsub.Manager, rootLogger zerolog.Logger, reg *metrics.Registry, healthMgr *health.CheckRegistry, testingMgr *testsupport.Manager, json jsoniter.API, clock clock.Clock) *Server {
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

	inboundSvcAuth, outboundSvcAuth, err := svcauth.LoadMethods(clock, runtime)
	if err != nil {
		panic(fmt.Errorf("error loading service auth methods: %w", err))
	}

	s := &Server{
		static:              static,
		runtime:             runtime,
		pc:                  pc,
		rt:                  rt,
		encoreMgr:           encoreMgr,
		pubsubMgr:           pubsubMgr,
		healthMgr:           healthMgr,
		testingMgr:          testingMgr,
		requestsTotal:       requestsTotal,
		httpClient:          &http.Client{},
		clock:               clock,
		rootLogger:          rootLogger,
		json:                json,
		tracingEnabled:      rt.TracingEnabled(),
		experiments:         experiments.FromConfig(static, runtime),
		functionsToHandlers: make(map[uintptr]Handler),

		public:           newRouter(),
		publicFallback:   newRouter(),
		private:          newRouter(),
		privateFallback:  newRouter(),
		encore:           newRouter(),
		inboundSvcAuth:   inboundSvcAuth,
		outboundSvcAuth:  outboundSvcAuth,
		remotePubSubPush: make(map[string]*httputil.ReverseProxy),
	}

	// Create our HTTP server handler chain

	// Start with the underlying router
	var baseHandler http.Handler = http.HandlerFunc(s.handler)

	// If we're acting as an API Gateway, then we need to add CORS support
	if s.IsGateway() {
		corsCfg := &config.CORS{}
		if runtime.CORS != nil {
			corsCfg = runtime.CORS
		}
		baseHandler = cors.Wrap(
			corsCfg,
			static.CORSAllowHeaders,
			static.CORSExposeHeaders,
			baseHandler,
			rootLogger,
		)
	}

	// Finally, this handler is used to track the number of running handlers
	// on the server so we can wait for them to finish before shutting down
	//
	// It must be the first handler in the chain to ensure the runningHandlers
	// count is always correct
	activeHandlersWrapper := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.httpCtx.Err() != nil {
			// We are shutting down, return 503 with a retry-after header to tell clients to back off
			// we shouldn't ever see this as `httpCtx` is only cancelled after the server has already
			// started shutting down, but this is here as a safety net just in case
			w.Header().Set("Retry-After", "2")
			errs.HTTPErrorWithCode(
				w,
				errs.B().Code(errs.Unavailable).Msg("server is shutting down").Err(),
				http.StatusServiceUnavailable,
			)

			return
		}

		// Now we can call the next handler in the chain
		s.runningHandlers.Add(1)
		defer s.runningHandlers.Done()

		baseHandler.ServeHTTP(w, r)
	})

	// Now we have the handler chain setup, create the HTTP server object
	s.httpCtx, s.httpCtxCancel = context.WithCancel(context.Background())
	s.httpsrv = &http.Server{
		Handler: h2c.NewHandler(activeHandlersWrapper, &http2.Server{}),
		BaseContext: func(_ net.Listener) context.Context {
			// We set the base context which allows us to cancel it when the server is shutting down
			return s.httpCtx
		},
	}

	s.configureRemotePubsubPush()
	s.registerEncoreRoutes()

	return s
}

// configureRemotePubsubPush adds pubsub push handlers for push subscriptions that are not hosted by this service.
// This is only done for gateway services.
func (s *Server) configureRemotePubsubPush() {
	if !s.IsGateway() {
		return
	}
	for _, topic := range s.runtime.PubsubTopics {
		statTop, ok := s.static.PubsubTopics[topic.EncoreName]
		if !ok {
			panic(fmt.Errorf("runtime topic %s not found in static config", topic.EncoreName))
		}
		for _, sub := range topic.Subscriptions {
			statSub, ok := statTop.Subscriptions[sub.EncoreName]
			if !ok {
				panic(fmt.Errorf("runtime sub %s/%s not found in static config", topic.EncoreName, sub.EncoreName))
			}
			if slices.Contains(s.runtime.HostedServices, statSub.Service) {
				continue
			}
			service, found := s.runtime.ServiceDiscovery[statSub.Service]
			if !found {
				panic(fmt.Errorf("service %q not found in service discovery, but needed for the remote push handler", statSub.Service))
			}
			var err error
			s.remotePubSubPush[sub.ID], err = s.createPubsubPushProxy(service)
			if err != nil {
				panic(fmt.Errorf("error creating remote pubsub push proxy: %w", err))
			}
		}
	}
}

// setAuthHandler sets the auth handler to use.
// If h is nil it means no auth handler is used.
func (s *Server) setAuthHandler(h AuthHandler) {
	authService := h.HostedByService()

	if !cfgutil.IsHostedService(s.runtime, authService) {
		service, found := s.runtime.ServiceDiscovery[authService]
		if !found {
			panic(fmt.Errorf("service %q not found in service discovery, but needed for the auth handler", authService))
		}

		authURL := fmt.Sprintf("%s/__encore/authhandler", service.URL)

		s.authHandler = &remoteAuthHandler{
			server:         s,
			hostingService: service,
			authURL:        authURL,
			original:       h,
			logger:         s.rootLogger.With().Str("auth_url", authURL).Logger(),
			traceLogs:      s.runtime.EnvCloud != "local", // log auth calls in prod containers only
		}
	} else {
		s.authHandler = h
	}
}

func (s *Server) RegisteredHandlers() []Handler {
	return s.registeredHandlers
}

// wildcardMethod is an internal method name we register wildcard methods under.
const wildcardMethod = "__ENCORE_WILDCARD__"

func (s *Server) registerEndpoint(h Handler, function any) {
	routerPath := h.HTTPRouterPath()

	// Decide which routers to use.
	private, public := s.private, s.public
	if h.IsFallback() {
		private, public = s.privateFallback, s.publicFallback
	}

	var adapter httprouter.Handle

	switch {
	case cfgutil.IsHostedService(s.runtime, h.ServiceName()):
		adapter = s.createServiceHandlerAdapter(h)

	case s.IsGateway():
		adapter = s.createGatewayHandlerAdapter(h)

	default:
		// not hosted do nothing
		return
	}

	s.registeredHandlers = append(s.registeredHandlers, h)

	// Register the adapter
	for _, m := range h.HTTPMethods() {
		if m == "*" {
			m = wildcardMethod
		}

		private.Handle(m, routerPath, adapter)
		if access := h.AccessType(); access == Public || access == RequiresAuth {
			public.Handle(m, routerPath, adapter)
		}
	}

	// Register the function mapped to the handler - this allows `et.MockEndpoint` to lookup the Handler
	// for a given function
	if s.static.Testing {
		if reflect.TypeOf(function).Kind() == reflect.Func {
			s.functionsToHandlers[reflect.ValueOf(function).Pointer()] = h
		} else {
			s.rootLogger.Warn().Str("service", h.ServiceName()).Str("endpoint", h.EndpointName()).Msgf("not registering function as lookup for API handler as it is not a function: %T", function)
		}
	}
}

// HandlerForFunc returns the Handler for the given function or nil if it does not exist.
func (s *Server) HandlerForFunc(function any) Handler {
	return s.functionsToHandlers[reflect.ValueOf(function).Pointer()]
}

// ServiceExists returns true if the given service exists and has at least one endpoint.
func (s *Server) ServiceExists(serviceName string) bool {
	for _, h := range s.registeredHandlers {
		if strings.EqualFold(h.ServiceName(), serviceName) {
			return true
		}
	}
	return false
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
	if s.runtime.EnvCloud != "local" || s.IsGateway() {
		s.rootLogger.Trace().Msg("listening for incoming HTTP requests")
	}
	return s.httpsrv.Serve(ln)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(p *shutdown.Process) error {
	// Once it's time to force-close tasks, cancel the base context.
	go func() {
		<-p.ForceCloseTasks.Done()
		s.httpCtxCancel()
	}()

	// Begin shutting down the server
	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- s.httpsrv.Shutdown(p.ForceShutdown) }()

	// Wait for the running handlers to finish.
	s.runningHandlers.Wait()
	p.MarkOutstandingRequestsCompleted()

	return <-shutdownErr
}

func (s *Server) handler(w http.ResponseWriter, req *http.Request) {
	// Select a router based on access
	router, fallbackRouter := s.public, s.publicFallback

	// The Encore platform is authorised to call private APIs directly, thus if we have this header set,
	// and authenticate it, then we can switch over to the private router which contains all APIs not just
	// the publicly accessible ones.
	if sig := req.Header.Get("X-Encore-Auth"); sig != "" && s.pc != nil {
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

	// Extract the call meta from the request
	req, internalCaller, ok := s.extractCallMeta(w, req)
	if !ok {
		// extractCallMeta has already written the response
		return
	}
	if internalCaller != nil && internalCaller.PrivateAPIAccess() {
		// If this request is from another service running in this app, allow it access to the private API routes
		router, fallbackRouter = s.private, s.privateFallback
	}

	path := determineRequestPath(req.URL)

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
	s.rootLogger.Trace().Str("path", path).Bool("gateway", s.IsGateway()).Strs("hosting", s.runtime.HostedServices).Msg("endpoint not found")
	errs.HTTPError(w, errs.B().Code(errs.NotFound).Msg("endpoint not found").Err())
}

func (s *Server) extractCallMeta(w http.ResponseWriter, req *http.Request) (updatedReq *http.Request, internalCaller Caller, ok bool) {
	// Extract the metadata from the request so we can allow access to the private router.
	// If the metadata is not present, then we assume this is a public request.
	meta, err := s.MetaFromRequest(transport.HTTPRequest(req))
	if err != nil {
		s.rootLogger.Error().Err(err).Msg("failed to extract metadata from request")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, nil, false
	}

	// Extract any cloud generated Trace identifiers from the request.
	// and use them if we don't have any trace information in the metadata already
	cloudGeneratedTraceIDs := cloudtrace.ExtractCloudTraceIDs(s.rootLogger, req)
	if meta.TraceID.IsZero() {
		meta.TraceID = cloudGeneratedTraceIDs.TraceID
	}

	// SpanID will be zero already, so if our Cloud generated one for us, we should
	// use it as the SpanID for this request
	meta.SpanID = cloudGeneratedTraceIDs.SpanID

	// If we still don't have a trace id, generate one.
	if meta.TraceID.IsZero() {
		meta.TraceID, _ = model.GenTraceID()
		meta.ParentSpanID = model.SpanID{} // no parent span if we have no trace id
	}

	var caller Caller
	if meta.Internal != nil {
		caller = meta.Internal.Caller
	}

	return req.WithContext(SetCallMetaInContext(req.Context(), meta)), caller, true
}

func (s *Server) newExecContext(ctx context.Context, ps UnnamedParams, callMeta CallMeta) execContext {
	var auth model.AuthInfo
	if callMeta.Internal != nil {
		auth = model.AuthInfo{
			UID:      model.UID(callMeta.Internal.AuthUID),
			UserData: callMeta.Internal.AuthData,
		}
	}
	return execContext{s, ctx, ps, auth, callMeta}
}

func (s *Server) NewIncomingContext(w http.ResponseWriter, req *http.Request, ps UnnamedParams, callMeta CallMeta) IncomingContext {
	ec := s.newExecContext(req.Context(), ps, callMeta)
	return IncomingContext{ec, w, req, nil}
}

func (s *Server) NewCallContext(ctx context.Context) CallContext {
	return CallContext{ctx, s}
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

// determineRequestPath determines the path to use for routing
// based on the incoming request URL u.
func determineRequestPath(u *url.URL) string {
	// To support use cases like routing "/foo%2Fbar/baz" to "/:a/*b" as a = "foo/bar", b = "baz"
	// we need to be careful about the escaping.
	//
	// The way the net/url package works is a bit subtle, but URL.RawPath is non-empty if and only if
	// the default encoding of Path differs from the incoming request.
	// However, we don't want to always use RawPath (or EscapedPath(), in practice) because
	// it over-escapes: it turns '{foo}' into '%7Bfoo%7D' which we don't want.
	//
	// So, use req.URL.Path when possible, and only use EscapedPath() when necessary.
	path := u.Path
	if u.RawPath != "" {
		path = u.EscapedPath()
	}
	return path
}
