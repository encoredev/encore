package objects

import (
	// nosemgrep

	"fmt"
	"net"
	"net/http"

	"encr.dev/cli/daemon/namespace"
	"encr.dev/pkg/emulators/storage/gcsemu"
	"github.com/cockroachdb/errors"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
	"go4.org/syncutil"

	meta "encr.dev/proto/encore/parser/meta/v1"
)

type Server struct {
	id        string
	public    *PublicBucketServer
	startOnce syncutil.Once
	cancel    func() // set by Start
	store     gcsemu.Store
	emu       *gcsemu.GcsEmu
	ln        net.Listener
	srv       *http.Server
	inMemory  bool
}

func NewInMemoryServer(public *PublicBucketServer) *Server {
	id := xid.New().String()
	store := gcsemu.NewMemStore()
	return newServer(public, id, store, true)
}

func NewDirServer(public *PublicBucketServer, nsID namespace.ID, baseDir string) *Server {
	store := gcsemu.NewFileStore(baseDir)
	return newServer(public, nsID.String(), store, false)
}

func newServer(public *PublicBucketServer, id string, store gcsemu.Store, isInMem bool) *Server {
	return &Server{
		public:   public,
		id:       id,
		store:    store,
		emu:      gcsemu.NewGcsEmu(gcsemu.Options{Store: store}),
		inMemory: isInMem,
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
		if s.inMemory {
			s.public.Register(s.id, s.store)
		}
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
	if s.inMemory {
		s.public.Deregister(s.id)
	}
}

func (s *Server) Endpoint() string {
	// Ensure the server has been started
	if err := s.Start(); err != nil {
		panic(err)
	}
	port := s.ln.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("http://localhost:%d", port)
}

func (s *Server) PublicBaseURL() string {
	return fmt.Sprintf("%s/%s", s.public.BaseAddr(), s.id)
}

// IsUsed reports whether the application uses object storage at all.
func IsUsed(md *meta.Data) bool {
	return len(md.Buckets) > 0
}
