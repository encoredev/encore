package objects

import (
	"net"
	"net/http"
	"strconv"
	"sync"

	"encr.dev/pkg/emulators/storage/gcsemu"
)

// Fallback is a function that returns a store for a given namespace.
// It is used for resolving namespace ids to stores, where
// the store is not pre-registered by Register.
type Fallback func(namespace string) (gcsemu.Store, bool)

// NewPublicBucketServer creates a new PublicBucketServer.
// If fallback is nil, no fallback will be used.
func NewPublicBucketServer(baseAddr string, fallback Fallback) *PublicBucketServer {
	mux := http.NewServeMux()
	srv := &PublicBucketServer{
		mux:        mux,
		baseAddr:   baseAddr,
		fallback:   fallback,
		namespaces: make(map[string]gcsemu.Store),
	}
	mux.HandleFunc("/{namespace}/{bucket}/{object...}", srv.handler)
	return srv
}

type PublicBucketServer struct {
	mux      *http.ServeMux
	baseAddr string
	fallback Fallback

	mu         sync.RWMutex
	namespaces map[string]gcsemu.Store
}

func (s *PublicBucketServer) Serve(ln net.Listener) error {
	return http.Serve(ln, s)
}

func (s *PublicBucketServer) Register(namespace string, store gcsemu.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.namespaces[namespace] = store
}

func (s *PublicBucketServer) Deregister(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.namespaces, namespace)
}

func (s *PublicBucketServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(w, req)
}

func (s *PublicBucketServer) BaseAddr() string {
	return s.baseAddr
}

func (s *PublicBucketServer) handler(w http.ResponseWriter, req *http.Request) {
	nsID := req.PathValue("namespace")
	bucketName := req.PathValue("bucket")
	objName := req.PathValue("object")

	// Determine which store to use
	s.mu.RLock()
	store, ok := s.namespaces[nsID]
	s.mu.RUnlock()
	if !ok && s.fallback != nil {
		store, ok = s.fallback(nsID)
	}
	if !ok {
		http.Error(w, "unknown namespace", http.StatusNotFound)
		return
	}

	obj, contents, err := store.Get("", bucketName, objName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if obj == nil {
		http.Error(w, "object not found", http.StatusNotFound)
		return
	}

	if obj.ContentType != "" {
		w.Header().Set("Content-Type", obj.ContentType)
	}
	if obj.Etag != "" {
		w.Header().Set("Etag", obj.Etag)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(contents)))
	w.Write(contents)
}
