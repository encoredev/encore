package objects

import (
	"context"

	"encore.dev/appruntime/exported/config"
	"encore.dev/storage/objects/internal/types"
)

type provider interface {
	ProviderName() string
	Matches(providerCfg *config.BucketProvider) bool
	NewBucket(providerCfg *config.BucketProvider, runtimeCfg *config.Bucket) types.BucketImpl
}

var providerRegistry []func(context.Context, *config.Runtime) provider

func registerProvider(p func(context.Context, *config.Runtime) provider) {
	providerRegistry = append(providerRegistry, p)
}
