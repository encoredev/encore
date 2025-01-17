package objects

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"encr.dev/pkg/emulators/storage/gcsemu"
	"google.golang.org/api/storage/v1"
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
	switch req.Method {
	case "GET":
		_, isSigned := (queryLowerCase(req))["x-goog-signature"]
		if isSigned {
			err := validateGcsSignedRequest(req, time.Now())
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
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
	case "PUT":
		err := validateGcsSignedRequest(req, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		buf, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		metaIn := parseObjectMeta(req)
		err = store.Add(bucketName, objName, buf, &metaIn)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Read back the object so we can add the etag value to the response.
		metaOut, _, err := store.Get("", bucketName, objName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Etag", metaOut.Etag)
	default:
		http.Error(w, "method not allowed", http.StatusBadRequest)
	}
}

// Only GCS is supported for local development
func validateGcsSignedRequest(req *http.Request, now time.Time) error {
	const dateLayout = "20060102T150405Z"
	const gracePeriod = time.Duration(30) * time.Second

	query := queryLowerCase(req)

	// We don't try to actually verify the signature, we only check that it's non-empty.

	for _, s := range []string{
		"x-goog-signature",
		"x-goog-credential",
		"x-goog-date",
		"x-goog-expires"} {
		if len(query[s]) <= 0 {
			return fmt.Errorf("missing or empty query param %q", s)
		}
	}

	t0, err := time.Parse(dateLayout, query["x-goog-date"])
	if err != nil {
		return errors.New("failed to parse x-goog-date")
	}
	if t0.After(now.Add(gracePeriod)) {
		return errors.New("URL expiration base date is in the future")
	}

	td, err := strconv.Atoi(query["x-goog-expires"])
	if err != nil {
		return errors.New("failed to parse x-goog-expires value into an integer")
	}
	t := t0.Add(time.Duration(td) * time.Second)

	if t.Before(now.Add(-gracePeriod)) {
		return errors.New("URL is expired")
	}

	return nil
}

func queryLowerCase(req *http.Request) map[string]string {
	query := map[string]string{}
	for k, vs := range req.URL.Query() {
		query[strings.ToLower(k)] = vs[0]
	}
	return query
}

func parseObjectMeta(req *http.Request) storage.Object {
	return storage.Object{ContentType: req.Header.Get("Content-Type")}
}
