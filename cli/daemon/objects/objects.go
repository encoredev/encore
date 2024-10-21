package objects

import (
	// nosemgrep

	"fmt"
	"net"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/fullstorydev/emulators/storage/gcsemu"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Server struct {
	startOnce syncutil.Once
	cancel    func() // set by Start
	emu       *gcsemu.GcsEmu
	ln        net.Listener
	srv       *http.Server
}

func New() *Server {
	return &Server{
		// TODO set up dir storage
		emu: gcsemu.NewGcsEmu(gcsemu.Options{
			Verbose: true,
			Log: func(err error, fmt string, args ...interface{}) {
				if err != nil {
					log.Error().Err(err).Msgf(fmt, args...)
				} else {
					log.Info().Msgf(fmt, args...)
				}
			},
		}),
	}
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

		s.emu.InitBucket("test")

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
