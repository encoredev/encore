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
	cm        *ClusterManager
	startOnce syncutil.Once
	cancel    func() // set by Start
	store     gcsemu.Store
	emu       *gcsemu.GcsEmu
	ln        net.Listener
	srv       *http.Server
	inMemory  bool
}

func NewInMemoryServer(cm *ClusterManager) *Server {
	id := xid.New().String()
	store := gcsemu.NewMemStore()
	return newServer(cm, id, store, true)
}

func NewDirServer(cm *ClusterManager, nsID namespace.ID, baseDir string) *Server {
	store := gcsemu.NewFileStore(baseDir)
	return newServer(cm, nsID.String(), store, false)
}

func newServer(cm *ClusterManager, id string, store gcsemu.Store, isInMem bool) *Server {
	return &Server{
		cm:       cm,
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
			s.cm.registerInMem(s)
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
		s.cm.deregisterInMem(s)
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
	return fmt.Sprintf("http://%s/%s", s.cm.publicAddr, s.id)
}

// IsUsed reports whether the application uses object storage at all.
func IsUsed(md *meta.Data) bool {
	return len(md.Buckets) > 0
}
