package gcsemu

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Server is an in-memory Cloud Storage emulator; it is unauthenticated, and only a rough approximation.
type Server struct {
	Addr string
	*httptest.Server
	*GcsEmu
}

// NewServer creates a new Server with the given options.
// The Server will be listening for HTTP connections, without TLS,
// on the provided address. The resolved address is named by the Addr field.
// An address with a port of 0 will bind to an open port on the system.
//
// For running a full in-process setup (e.g. unit tests), initialize
// os.Setenv("GCS_EMULATOR_HOST", srv.Addr) so that subsequent calls to NewClient()
// will return an in-process targeted storage client.
func NewServer(laddr string, opts Options) (*Server, error) {
	gcsEmu := NewGcsEmu(opts)
	mux := http.NewServeMux()
	gcsEmu.Register(mux)

	srv := httptest.NewUnstartedServer(mux)
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on addr %s: %w", laddr, err)
	}
	srv.Listener = l
	srv.Start()

	return &Server{
		Addr:   strings.TrimPrefix(srv.URL, "http://"),
		Server: srv,
		GcsEmu: gcsEmu,
	}, nil
}
