//go:build !encore_no_azure

package objects

import (
	"context"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/providers/azblob"
)

func init() {
	registerProvider(func(ctx context.Context, runtimeCfg *config.Runtime) provider {
		return azblob.NewManager(ctx, runtimeCfg)
	})
}
