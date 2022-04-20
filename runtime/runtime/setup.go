package runtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/common/expfmt"
	"github.com/rs/zerolog"

	"encore.dev/internal/metrics"
	"encore.dev/runtime/config"
)

var defaultServer = setup()

func ListenAndServe() error {
	return defaultServer.ListenAndServe()
}

type Server struct {
	logger  zerolog.Logger
	public  *httprouter.Router
	private *httprouter.Router
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
	httpsrv := &http.Server{
		Handler: http.HandlerFunc(srv.handler),
	}
	srv.logger.Info().Msg("listening for incoming HTTP requests")
	return httpsrv.Serve(ln)
}

func (srv *Server) handler(w http.ResponseWriter, req *http.Request) {
	ep := strings.TrimPrefix(req.URL.Path, "/")
	if strings.HasPrefix(ep, "__encore/") {
		// TODO this should only run for authenticated requests.
		api := ep[len("__encore/"):]
		switch api {
		case "healthz":
			srv.healthz(w, req)
		default:
			http.Error(w, "unknown internal endpoint: "+ep, http.StatusNotFound)
		}
		return
	}

	// Select a router based on access
	r := srv.public
	if h := req.Header.Get("X-Encore-Auth"); h != "" {
		if ok, err := srv.checkAuth(req, h); err == nil && ok {
			// Sucessfully authenticated
			r = srv.private
		} else if err != nil {
			http.Error(w, "could not authenticate request", http.StatusBadGateway)
			return
		} else {
			http.Error(w, "invalid request signature", http.StatusUnauthorized)
			return
		}
	}

	h, p, _ := r.Lookup(req.Method, req.URL.Path)
	if h == nil {
		h, p, _ = r.Lookup(wildcardMethod, req.URL.Path)
	}
	if h == nil {
		svc, api := "unknown", "Unknown"
		if idx := strings.IndexByte(ep, '.'); idx != -1 {
			svc, api = ep[:idx], ep[idx+1:]
		}
		metrics.UnknownEndpoint(svc, api)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{
  "code": "unknown_endpoint",
  "message": "endpoint not found",
  "details": null
}
`))
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

func (srv *Server) healthz(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
  "message": "Your Encore app is up and running!"
}
`))
}

func (srv *Server) checkAuth(req *http.Request, macSig string) (bool, error) {
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
	for _, k := range config.Cfg.Runtime.AuthKeys {
		if k.KeyID == keyID {
			return checkAuth(k, req, mac), nil
		}
	}

	return false, nil
}

func setup() *Server {
	configureZerologOutput()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	RootLogger = &logger

	public := httprouter.New()
	public.HandleOPTIONS = false
	public.RedirectFixedPath = false
	public.RedirectTrailingSlash = false

	private := httprouter.New()
	private.HandleOPTIONS = false
	private.RedirectFixedPath = false
	private.RedirectTrailingSlash = false

	srv := &Server{
		logger:  logger,
		public:  public,
		private: private,
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
