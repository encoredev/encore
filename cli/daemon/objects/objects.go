package objects

import (
	// nosemgrep

	"fmt"
	"net"
	"net/http"

	"encr.dev/pkg/emulators/storage/gcsemu"
	"github.com/cockroachdb/errors"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Server struct {
	cm        *ClusterManager
	startOnce syncutil.Once
	cancel    func() // set by Start
	emu       *gcsemu.GcsEmu
	ln        net.Listener
	srv       *http.Server
}

func NewInMemoryServer() *Server {
	return &Server{
		// TODO set up dir storage
		emu: gcsemu.NewGcsEmu(gcsemu.Options{
			Store: gcsemu.NewMemStore(),
		}),
	}
}

func NewDirServer(baseDir string) *Server {
	return &Server{
		emu: gcsemu.NewGcsEmu(gcsemu.Options{
			Store: gcsemu.NewFileStore(baseDir),
		}),
	}
}

func (s *Server) Initialize(md *meta.Data) error {
	for _, bucket := range md.Buckets {
		if err := s.emu.InitBucket(bucket.Name); err != nil {
			return errors.Wrap(err, "initialize object storage bucket")
		}
	}
	return nil
}

func (s *Server) Start() error {
	return s.startOnce.Do(func() error {
		mux := http.NewServeMux()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return errors.Wrap(err, "listen tcp")
		}
		s.emu.Register(mux)
		s.ln = ln
		s.srv = &http.Server{Handler: mux}

		go func() {
			if err := s.srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				log.Error().Err(err).Msg("unable to listen to gcs server")
			}
		}()

		return nil
	})
}

func (s *Server) Stop() {
	_ = s.srv.Close()
}

func (s *Server) Endpoint() string {
	// Ensure the server has been started
	if err := s.Start(); err != nil {
		panic(err)
	}
	port := s.ln.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("http://localhost:%d", port)
}

// IsUsed reports whether the application uses object storage at all.
func IsUsed(md *meta.Data) bool {
	return len(md.Buckets) > 0
}
