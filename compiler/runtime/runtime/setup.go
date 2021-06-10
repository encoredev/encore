package runtime

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"encore.dev/runtime/config"
	"github.com/hashicorp/yamux"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

type Server struct {
	logger zerolog.Logger
	router *httprouter.Router
}

func (srv *Server) handleRPC(service string, endpoint *config.Endpoint) {
	logMsg := srv.logger.Info().Str("service", service).Str("endpoint", endpoint.Name)
	if endpoint.Path != "" {
		logMsg.Str("path", endpoint.Path)
	}
	logMsg.Msg("registered endpoint")

	key := fmt.Sprintf("/%s.%s", service, endpoint.Name)
	srv.router.Handle("*", filepath.Join(key, endpoint.Path), func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		endpoint.Handler(w, req)
	})
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
	handler, _, ok := srv.router.Lookup("*", req.URL.Path)
	if !ok {
		http.Error(w, "Endpoint Not Found", http.StatusNotFound)
		return
	}
	handler(w, req, nil)
}

func Setup(cfg *config.ServerConfig) *Server {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	RootLogger = &logger
	Config = cfg

	srv := &Server{
		logger: logger,
		router: httprouter.New(),
	}
	for _, svc := range cfg.Services {
		for _, endpoint := range svc.Endpoints {
			srv.handleRPC(svc.Name, endpoint)
		}
	}
	return srv
}

type dummyAddr struct{}

func (dummyAddr) Network() string {
	return "encore"
}

func (dummyAddr) String() string {
	return "encore://localhost"
}
