package runtime

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/yamux"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"

	"encore.dev/runtime/config"
)

var defaultServer = setup()

func ListenAndServe() error {
	return defaultServer.ListenAndServe()
}

type Server struct {
	logger zerolog.Logger
	router *httprouter.Router
}

// wildcardMethod is an internal method name we register wildcard methods under.
const wildcardMethod = "__ENCORE_WILDCARD__"

func (srv *Server) handleRPC(service string, endpoint *config.Endpoint) {
	logMsg := srv.logger.Info().Str("func", service+"."+endpoint.Name).Str("path", endpoint.Path)
	logMsg.Msgf("registered endpoint")
	for _, m := range endpoint.Methods {
		if m == "*" {
			m = wildcardMethod
		}
		srv.router.Handle(m, endpoint.Path, endpoint.Handler)
	}
}

func (srv *Server) ListenAndServe() error {
	rwc, err := srv.setupConn()
	if err != nil {
		return err
	}
	s, err := yamux.Server(rwc, yamux.DefaultConfig())
	if err != nil {
		return err
	}
	httpsrv := &http.Server{
		Handler: http.HandlerFunc(srv.handler),
	}
	return httpsrv.Serve(s)
}

func (srv *Server) setupConn() (io.ReadWriteCloser, error) {
	var in, out *os.File
	if runtime.GOOS == "windows" {
		extraFiles := os.Getenv("ENCORE_EXTRA_FILES")
		fds := strings.Split(extraFiles, ",")
		if len(fds) < 2 {
			return nil, fmt.Errorf("could not get request/response file descriptors: %q", extraFiles)
		}
		infd, err1 := strconv.Atoi(fds[0])
		outfd, err2 := strconv.Atoi(fds[1])
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("could not parse request/response file descriptors: %q", extraFiles)
		}
		in = os.NewFile(uintptr(infd), "encore-stdin")
		out = os.NewFile(uintptr(outfd), "encore-stdout")
	} else {
		in = os.NewFile(uintptr(3), "encore-stdin")
		out = os.NewFile(uintptr(4), "encore-stdout")
	}

	rwc := struct {
		io.Reader
		io.WriteCloser
	}{
		Reader:      in,
		WriteCloser: out,
	}
	return rwc, nil
}

func (srv *Server) handler(w http.ResponseWriter, req *http.Request) {
	h, p, _ := srv.router.Lookup(req.Method, req.URL.Path)
	if h == nil {
		h, p, _ = srv.router.Lookup(wildcardMethod, req.URL.Path)
	}
	if h == nil {
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

func setup() *Server {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	RootLogger = &logger

	router := httprouter.New()
	router.HandleOPTIONS = false
	router.RedirectFixedPath = false
	router.RedirectTrailingSlash = false

	srv := &Server{
		logger: logger,
		router: router,
	}
	for _, svc := range config.Cfg.Static.Services {
		for _, endpoint := range svc.Endpoints {
			srv.handleRPC(svc.Name, endpoint)
		}
	}
	return srv
}
