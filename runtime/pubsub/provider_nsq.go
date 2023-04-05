//go:build !encore_no_local

package pubsub

import (
	"encore.dev/pubsub/internal/nsq"
)

func init() {
	registerProvider(func(mgr *Manager) provider {
		return nsq.NewManager(mgr.ctx, mgr.rt)
	})
}
