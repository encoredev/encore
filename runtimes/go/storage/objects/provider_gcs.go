//go:build !encore_no_gcp || !encore_no_encorecloud || !encore_no_local

package objects

import (
	"context"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/providers/gcs"
)

func init() {
	registerProvider(func(ctx context.Context, runtimeCfg *config.Runtime) provider {
		return gcs.NewManager(ctx, runtimeCfg)
	})
}
