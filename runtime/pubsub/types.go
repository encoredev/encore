package pubsub

import (
	"context"
	"time"

	"encore.dev/pubsub/internal/types"
)

// SubscriptionConfig is used when creating a subscription
type SubscriptionConfig[T any] struct {
	// The function which will be called to process a message
	// sent on the topic.
	//
	// It is important for this function to block and not return
	// until all processing relating to the message has been completed.
	//
	// When this function returns a `nil`, the message will be
	// acknowledged (acked) from the topic, and should not be redelivered.
	//
	// When this function returns an `error`, the message will be
	// negatively acknowledged (nacked), which will cause a redelivery
	// attempt to be made (unless the retry policy's MaxRetries has been reached).
	//
	// This field is required.
	Handler func(ctx context.Context, msg T) error

	// Filter is a boolean expression using =, !=, IN, &&
	// It is used to filter which messages are forwarded from the
	// topic to a subscription
	// Filter string - Filters are not currently supported

	// AckDeadline is the time a consumer has to process a message
	// before it's returned to the subscription
	AckDeadline time.Duration

	// MessageRetention is how long an undelivered message is kept
	// on the topic before it's purged
	MessageRetention time.Duration

	// RetryPolicy defines how a message should be retried when
	// the subscriber returns an error
	RetryPolicy *RetryPolicy
}

type RetryPolicy = types.RetryPolicy

const (
	NoRetries       = types.NoRetries
	InfiniteRetries = types.InfiniteRetries
)

type DeliveryGuarantee = types.DeliveryGuarantee

const (
	AtLeastOnce = types.AtLeastOnce
)

type TopicConfig = types.TopicConfig
