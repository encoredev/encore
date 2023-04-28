//go:build !encore_no_encorecloud

package pubsub

import (
	"encore.dev/pubsub/internal/encorecloud"
)

func init() {
	registerProvider(func(mgr *Manager) provider {
		return encorecloud.NewManager(mgr.ctx, mgr.runtime, mgr)
	})
}
