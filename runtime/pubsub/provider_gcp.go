//go:build !encore_no_gcp

package pubsub

import (
	"encore.dev/pubsub/internal/gcp"
)

func init() {
	registerProvider(func(mgr *Manager) provider {
		return gcp.NewManager(mgr.ctx, mgr.runtime, mgr)
	})
}
