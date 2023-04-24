package encorecloud

import (
	"context"

	"encore.dev/appruntime/exported/config"
	"encore.dev/beta/errs"
	"encore.dev/internal/ecauth"
	"encore.dev/pubsub/internal/types"
)

type Manager struct {
	ctx          context.Context
	runtime      *config.Runtime
	pushRegistry types.PushEndpointRegistry
}

func NewManager(ctx context.Context, runtime *config.Runtime, pushRegistry types.PushEndpointRegistry) *Manager {
	return &Manager{ctx: ctx, runtime: runtime, pushRegistry: pushRegistry}
}

func (mgr *Manager) ProviderName() string {
	return "encorecloud"
}

func (mgr *Manager) Matches(providerCfg *config.PubsubProvider) bool {
	return providerCfg.EncoreCloud != nil
}

func (mgr *Manager) NewTopic(_ *config.PubsubProvider, topicCfg *config.PubsubTopic) types.TopicImplementation {
	return &topic{mgr, topicCfg}
}

// latestAuthKey returns the latest auth key for EncoreCloud pubsub
func (mgr *Manager) latestAuthKey() (latest ecauth.Key, err error) {
	for _, key := range mgr.runtime.EncoreCloudAPI.AuthKeys {
		if key.KeyID > latest.KeyID {
			latest = key
		}
	}

	if latest.KeyID == 0 {
		err = errs.B().Code(errs.FailedPrecondition).Msg("no auth keys found for encorecloud pubsub").Err()
	}
	return latest, err
}
