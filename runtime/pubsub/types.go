package pubsub

import (
	"context"
	"time"

	"encore.dev/pubsub/internal/types"
)

// parentTraceIDAttribute is the attribute name we use to track request correlation IDs
const parentTraceIDAttribute = "encore_parent_trace_id"

// extCorrelationIDAttribute is the attribute name we use to track externally provided correlation IDs
const extCorrelationIDAttribute = "encore_ext_correlation_id"

// SubscriptionConfig is used when creating a subscription
//
// The values given here may be clamped to the supported values by
// the target cloud. (i.e. ack deadline may be brought within the supported range
// by the target cloud pubsub implementation).
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
	//
	// Default is 30 seconds, however the ack deadline must be at least
	// 1 second.
	AckDeadline time.Duration

	// MessageRetention is how long an undelivered message is kept
	// on the topic before it's purged
	// Default is 7 days.
	MessageRetention time.Duration

	// RetryPolicy defines how a message should be retried when
	// the subscriber returns an error
	RetryPolicy *RetryPolicy
}

type RetryPolicy = types.RetryPolicy

const (
	NoRetries = types.NoRetries

	InfiniteRetries = types.InfiniteRetries
)

type DeliveryGuarantee = types.DeliveryGuarantee

const (
	AtLeastOnce = types.AtLeastOnce

	ExactlyOnce = types.ExactlyOnce
)

type TopicConfig = types.TopicConfig
