//go:build !encore_no_gcp && !encore_no_encorecloud

package objects

import (
	"encore.dev/storage/objects/internal/gcp"
)

func init() {
	registerProvider(func(mgr *Manager) provider {
		return gcp.NewManager(mgr.ctx, mgr.static, mgr.runtime)
	})
}
