package noop

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/exported/config"
	"encore.dev/pubsub/internal/types"
)

type Topic struct{}

var _ types.TopicImplementation = (*Topic)(nil)

var ErrNoop = errors.New(
	"pubsub: this service is not configured to use this topic. " +
		"Use pubsub.TopicRef in the service to get a reference and access to the topic from this service",
)

func (t *Topic) PublishMessage(ctx context.Context, orderingKey string, attrs map[string]string, data []byte) (id string, err error) {
	return "", ErrNoop
}

func (t *Topic) Subscribe(logger *zerolog.Logger, maxConcurrency int, _ time.Duration, _ *types.RetryPolicy, subCfg *config.PubsubSubscription, f types.RawSubscriptionCallback) {
	// no-op
}
