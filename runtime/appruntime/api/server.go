package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
	"encore.dev/appruntime/cors"
	"encore.dev/appruntime/model"
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
	rootLogger zerolog.Logger
	json       jsoniter.API

	public  *httprouter.Router
	private *httprouter.Router
	encore  *httprouter.Router
	httpsrv *http.Server

	callCtr uint64

	pubsubSubscriptions map[string]func(r *http.Request) error
}

func NewServer(cfg *config.Config, rt *reqtrack.RequestTracker, rootLogger zerolog.Logger, json jsoniter.API) *Server {
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
		rootLogger: rootLogger,
		rt:         rt,
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
			h.Handle(Context{
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
	// and authenticate it, then we can switch over to the private router which contains all API's not just
	// the publicly accessible ones.
	if h := req.Header.Get("X-Encore-Auth"); h != "" {
		if ok, err := s.authPlatformReq(req, h); err == nil && ok {
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

// healthz serves the /__encore/healthz health checking endpoint.
func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	bytes, _ := json.Marshal(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details any    `json:"details"`
	}{
		Code:    "ok",
		Message: "Your Encore app is up and running!",
		Details: struct {
			AppRevision    string `json:"app_revision"`
			EncoreCompiler string `json:"encore_compiler"`
			DeployId       string `json:"deploy_id"`
		}{
			AppRevision:    s.cfg.Static.AppCommit.AsRevisionString(),
			EncoreCompiler: s.cfg.Static.EncoreCompiler,
			DeployId:       s.cfg.Runtime.DeployID,
		},
	})
	_, _ = w.Write(bytes)
}

func (s *Server) authPlatformReq(req *http.Request, macSig string) (bool, error) {
	macBytes, err := base64.RawStdEncoding.DecodeString(macSig)
	if err != nil {
		return false, nil
	}

	// Pull out key ID from hmac prefix
	const keyIDLen = 4
	if len(macBytes) < keyIDLen {
		return false, nil
	}

	keyID := binary.BigEndian.Uint32(macBytes[:keyIDLen])
	mac := macBytes[keyIDLen:]
	for _, k := range s.cfg.Runtime.AuthKeys {
		if k.KeyID == keyID {
			return checkAuthKey(k, req, mac), nil
		}
	}

	return false, nil
}

func checkAuthKey(key config.EncoreAuthKey, req *http.Request, gotMac []byte) bool {
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
