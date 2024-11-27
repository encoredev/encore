package objects

import (
	"context"
	"os"
	"path/filepath"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
	"encr.dev/pkg/emulators/storage/gcsemu"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager(ns *namespace.Manager) *ClusterManager {
	mgr := &ClusterManager{
		ns: ns,
	}
	return mgr
}

type ClusterManager struct {
	ns *namespace.Manager
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

// PersistentStoreFallback is a public server fallback handler
// for resolving stores based on the cluster manager's base directory.
func (cm *ClusterManager) PersistentStoreFallback(id string) (gcsemu.Store, bool) {
	if baseDir, err := cm.BaseDir(namespace.ID(id)); err == nil {
		if _, err := os.Stat(baseDir); err == nil {
			return gcsemu.NewFileStore(baseDir), true
		}
	}
	return nil, false
}
