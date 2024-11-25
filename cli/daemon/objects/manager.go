package objects

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/pkg/emulators/storage/gcsemu"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager(ns *namespace.Manager, publicAddr string) *ClusterManager {
	return &ClusterManager{
		ns:          ns,
		publicAddr:  publicAddr,
		inMemStores: make(map[string]*Server),
	}
}

type ClusterManager struct {
	ns         *namespace.Manager
	publicAddr string

	inMemMu     sync.RWMutex
	inMemStores map[string]*Server
}

func (cm *ClusterManager) BaseDir(ns namespace.ID) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cache, "encore", "objects", ns.String()), nil
}

// CanDeleteNamespace implements namespace.DeletionHandler.
func (cm *ClusterManager) CanDeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	return nil
}

// DeleteNamespace implements namespace.DeletionHandler.
func (cm *ClusterManager) DeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	baseDir, err := cm.BaseDir(ns.ID)
	if err == nil {
		err = os.RemoveAll(baseDir)
	}
	return err
}

func (cm *ClusterManager) ServePublic(ln net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/{namespace}/{bucket}/{object...}", cm.publicHandler)
	return http.Serve(ln, mux)
}

func (cm *ClusterManager) registerInMem(s *Server) {
	cm.inMemMu.Lock()
	defer cm.inMemMu.Unlock()
	cm.inMemStores[s.id] = s
}

func (cm *ClusterManager) deregisterInMem(s *Server) {
	cm.inMemMu.Lock()
	defer cm.inMemMu.Unlock()
	delete(cm.inMemStores, s.id)
}

func (cm *ClusterManager) getInMem(id string) (*Server, bool) {
	cm.inMemMu.RLock()
	defer cm.inMemMu.RUnlock()
	s, ok := cm.inMemStores[id]
	return s, ok
}

func (cm *ClusterManager) publicHandler(w http.ResponseWriter, req *http.Request) {
	nsID := req.PathValue("namespace")
	bucketName := req.PathValue("bucket")
	objName := req.PathValue("object")

	// Determine which store to use
	var store gcsemu.Store
	if s, ok := cm.getInMem(nsID); ok {
		store = s.store
	} else if nsID, ok := namespace.ParseID(nsID); ok {
		dir, err := cm.BaseDir(nsID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		store = gcsemu.NewFileStore(dir)
	} else {
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
