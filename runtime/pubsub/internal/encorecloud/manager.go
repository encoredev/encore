package encorecloud

import (
	"context"

	"go.encore.dev/platform-sdk"
	"go.encore.dev/platform-sdk/encorecloud"
	"go.encore.dev/platform-sdk/pkg/auth"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
)

type Manager struct {
	ctx          context.Context
	client       *encorecloud.Client
	pushRegistry types.PushEndpointRegistry
}

func NewManager(ctx context.Context, runtime *config.Runtime, pushRegistry types.PushEndpointRegistry) *Manager {
	// It's possible that the runtime is nil, for example if the app isn't using this manager
	// so we need to check for that.
	server := ""
	var authKeys []auth.Key
	if runtime.EncoreCloudAPI != nil {
		server = runtime.EncoreCloudAPI.Server
		authKeys = runtime.EncoreCloudAPI.AuthKeys
	}

	sdk := platform.NewSDK(
		platform.WithHost(server),
		platform.WithAppDetails(runtime.AppSlug, runtime.EnvName),
		platform.WithAuthKeys(authKeys...),
	)
	return &Manager{ctx: ctx, client: sdk.EncoreCloud, pushRegistry: pushRegistry}
}

func (mgr *Manager) ProviderName() string {
	return "encorecloud"
}

func (mgr *Manager) Matches(providerCfg *config.PubsubProvider) bool {
	return providerCfg.EncoreCloud != nil
}

func (mgr *Manager) NewTopic(_ *config.PubsubProvider, _ types.TopicConfig, runtimeCfg *config.PubsubTopic) types.TopicImplementation {
	return &topic{mgr, runtimeCfg}
}
