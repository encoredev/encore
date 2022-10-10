package types

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"encore.dev/appruntime/config"
)

// RawSubscriptionCallback represents a unified callback structure allowing us to create a standardised callback for each implementation
type RawSubscriptionCallback func(ctx context.Context, msgID string, publishTime time.Time, deliveryAttempt int, attrs map[string]string, data []byte) error

// TopicImplementation gives us a private API to implementing topics, which we can change without impacting the public API
type TopicImplementation interface {
	PublishMessage(ctx context.Context, attrs map[string]string, data []byte) (id string, err error)
	Subscribe(logger *zerolog.Logger, ackDeadline time.Duration, retryPolicy *RetryPolicy, implCfg *config.PubsubSubscription, f RawSubscriptionCallback)
}
