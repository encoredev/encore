package objects

import (
	"context"
	"os"
	"path/filepath"

	"encr.dev/cli/daemon/apps"
	"encr.dev/cli/daemon/namespace"
)

// NewClusterManager creates a new ClusterManager.
func NewClusterManager(ns *namespace.Manager) *ClusterManager {
	return &ClusterManager{
		ns: ns,
	}
}

type ClusterManager struct {
	ns *namespace.Manager
}

func (cm *ClusterManager) BaseDir(ctx context.Context, ns *namespace.Namespace) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cache, "encore", "objects", ns.ID.String()), nil
}

// CanDeleteNamespace implements namespace.DeletionHandler.
func (cm *ClusterManager) CanDeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	return nil
}

// DeleteNamespace implements namespace.DeletionHandler.
func (cm *ClusterManager) DeleteNamespace(ctx context.Context, app *apps.Instance, ns *namespace.Namespace) error {
	baseDir, err := cm.BaseDir(ctx, ns)
	if err == nil {
		err = os.RemoveAll(baseDir)
	}
	return err
}
