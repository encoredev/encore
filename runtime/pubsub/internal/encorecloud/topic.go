package encorecloud

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
)

type topic struct {
	mgr *Manager
	cfg *config.PubsubTopic
}

func (t *topic) PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error) {
	return t.mgr.client.PublishToTopic(ctx, t.cfg.ProviderName, attrs, data)
}

func (t *topic) Subscribe(logger *zerolog.Logger, _ time.Duration, _ *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	if subCfg.ID == "" {
		panic("encorecloud pubsub subscriptions must have an ID")
	}

	// registerPushEndpoint registers a push endpoint for a subscription from Encore Cloud
	t.mgr.pushRegistry.RegisterPushSubscriptionHandler(
		types.SubscriptionID(subCfg.ID),
		t.mgr.client.CreateSubscriptionHandler(subCfg.ID, logger, f),
	)
}
