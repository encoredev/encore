package noopgateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
)

// ServiceName is the name of a service.
type ServiceName string

// Route defines a single route in the gateway.
type Route struct {
	Methods []string    // HTTP methods
	Path    string      // "/path/:param/*wildcard"
	Dest    ServiceName // where to route the request

	// RequiresAuth specifies whether the route requires authentication.
	RequiresAuth bool
}

// Service describes how to route traffic to a service.
type Service struct {
	// URL is the URL to proxy traffic to.
	URL *url.URL
}

// AuthHandler describes the auth handler.
type AuthHandler struct {
	// Service is the service containing the auth handler.
	Service ServiceName
	Name    string // name of the auth handler

	// Parameters that the auth handler expects.
	// If any one of these are present in the request,
	// the auth handler is invoked.
	Query  []Param
	Header []Param
	Cookie []Param
}

// Param describes a request parameter.
type Param struct {
	// WireFormat is the name of the parameter on the wire.
	WireFormat string
	// CaseSensitive specifies whether or not
	// the wire format is case sensitive.
	CaseSensitive bool
}

// Description describes a gateway.
type Description struct {
	// Routes are the routes to proxy.
	Routes []*Route

	// Services defines how to proxy traffic to each service.
	Services map[ServiceName]Service

	// Auth describes the authentication handler, if any.
	Auth *AuthHandler
}

// New constructs a new gateway.
func New(desc *Description) *Gateway {
	// TODO validate config:
	// route -> service mapping
	// requires auth -> auth handler present

	gw := &Gateway{
		RoundTripper: http.DefaultTransport,
		desc:         desc,
		routeLookup:  newRouteLookuper(desc.Routes),
	}

	// Create our proxy.
	gw.proxy = &httputil.ReverseProxy{
		Rewrite:      gw.handleRequest,
		Transport:    &errorCheckingRoundTripper{gw: gw},
		ErrorHandler: gw.errorHandler,
	}

	return gw
}

// Gateway implements a gateway that validates and authenticates incoming requests,
// and forwards them to the appropriate service.
type Gateway struct {
	// Rewrite allows for rewriting the request before it is proxied.
	// If nil it does nothing.
	Rewrite func(p *httputil.ProxyRequest)

	// RoundTripper is the http.RoundTripper to use. It defaults to http.DefaultTransport.
	RoundTripper http.RoundTripper

	// ExtraRoutes, if non-nil, specifies a router that will be consulted first.
	// Requests will be proxied to the backend only if the router has no matching path.
	ExtraRoutes *mux.Router

	desc *Description

	// routeLookup is the httprouter for resolving incoming requests
	// to the routes they match.
	routeLookup *httprouter.Router

	// proxy proxies requests to the right backend.
	proxy *httputil.ReverseProxy
}

// ServeHTTP implements http.Handler.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if g.ExtraRoutes != nil {
		var match mux.RouteMatch
		if g.ExtraRoutes.Match(req, &match) {
			// ExtraRoutes has a matching route. Use it.
			g.ExtraRoutes.ServeHTTP(w, req)
			return
		}
	}

	g.proxy.ServeHTTP(w, req)
}

// handleRequest handles the request.
func (g *Gateway) handleRequest(r *httputil.ProxyRequest) {
	// setErrResp sets an error response, by setting the
	// sentinel error.
	setErrResp := func(err error) {
		out := &http.Request{
			// Assumed to be non-nil by httputil.
			Header: make(http.Header),
		}
		ctx := context.WithValue(context.Background(), errCtxKey, err)
		r.Out = out.WithContext(ctx)
	}

	route, ok := g.lookupRoute(r.In)
	if !ok {
		setErrResp(errRouteNotFound)
		return
	}

	// TODO perform authentication

	dest := g.desc.Services[route.Dest]
	r.SetURL(dest.URL)

	if g.Rewrite != nil {
		g.Rewrite(r)
	}
}

// lookupRoute looks up the route the request is for.
// If no route is found, it reports (nil, false).
func (g *Gateway) lookupRoute(req *http.Request) (route *Route, ok bool) {
	handle, _, tsr := g.routeLookup.Lookup(req.Method, req.URL.Path)

	// Handle trailing slash redirects.
	if tsr {
		path := req.URL.Path
		if strings.HasSuffix(path, "/") {
			path = path[:len(path)-1]
		} else {
			path += "/"
		}
		handle, _, _ = g.routeLookup.Lookup(req.Method, path)
	}

	if handle == nil {
		// Route not found
		return nil, false
	}

	rw := &sentinelResponseWriter{}
	handle(rw, nil, httprouter.Params{})
	return rw.route, true
}

func (g *Gateway) errorHandler(w http.ResponseWriter, req *http.Request, err error) {
	// TODO handle this properly
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

var (
	errRouteNotFound = errors.New("route not found")
)

// sentinelResponseWriter is a sentinel value that implements
// http.ResponseWriter as a way to get the Route registered for a given path.
type sentinelResponseWriter struct {
	http.ResponseWriter // always nil; dummy value to implement http.ResponseWriter
	// route is the selected route (an output parameter).
	route *Route
}

// errorCheckingRoundTripper is a http.RoundTripper that
// checks for specific error responses in the request context
// and returns an error if they are found.
type errorCheckingRoundTripper struct {
	gw *Gateway
}

func (rt errorCheckingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if err, ok := ctx.Value(errCtxKey).(error); ok && err != nil {
		return nil, err
	}
	return rt.gw.RoundTripper.RoundTrip(req)
}

type ctxKey string

const errCtxKey ctxKey = "error"

func newRouteLookuper(routes []*Route) *httprouter.Router {
	r := httprouter.New()
	for _, route := range routes {
		route := route // for the closure below
		for _, m := range route.Methods {
			r.Handle(m, route.Path, func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
				w.(*sentinelResponseWriter).route = route
			})
		}
	}
	return r
}

// authInfo represents extracted auth information from a request.
type authInfo struct {
}

// extractAuthInfo extracts the auth information from the request.
// If there is no auth information (or the gateway has no auth handler)
// it reports nil.
func (g *Gateway) extractAuthInfo(req *http.Request) *authInfo {
	auth := g.desc.Auth
	if auth == nil {
		return nil
	}

	// TODO check Query, Header, Cookie etc
	// TODO check legacy bearer token auth

	return nil
}
