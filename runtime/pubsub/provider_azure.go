//go:build !encore_no_azure

package pubsub

import "encore.dev/pubsub/internal/azure"

func init() {
	registerProvider(func(mgr *Manager) provider {
		return azure.NewManager(mgr.ctxs)
	})
}
