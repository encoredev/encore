package types

import (
	"context"
	"time"
)

// SubscriptionConfig is used when creating a subscription
type SubscriptionConfig struct {
	// Filter is a boolean expression using =, !=, IN, &&
	// It is used to filter which messages are forwarded from the
	// topic to a subscription
	Filter string

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

type RetryPolicy struct {
	// If MaxRetryDelay >= MinRetryDelay the first retry will be
	// delayed MinRetryDelayed and then following retries will backoff
	// exponentially until reaching the MaxRetryDelay
	MinRetryDelay time.Duration
	MaxRetryDelay time.Duration
	// MaxRetries is used to control deadletter queuing logic
	// n = -1 does not create a DLQ and retries infinitely
	// n >=0 creates a DLQ and forwards a message to it after n retries
	MaxRetries int
}

// Subscriber is a function reference
// The signature must be `func(context.Context, msg M) error` where M is either the
// message type of the topic or RawMessage
type Subscriber[T any] func(ctx context.Context, msg T) error

type Subscription[T any] interface{}

// DeliveryGuarantee is used to configure the delivery contract for a topic
type DeliveryGuarantee int

const (
	// AtLeastOnce guarantees that a message for a subscription is delivered to
	// a subscriber at least once
	AtLeastOnce DeliveryGuarantee = iota
	// ExactlyOnce guarantees that a message for a subscription is delivered to
	// a subscriber exactly once
	ExactlyOnce
)

// DeliveryPolicy configures how messages are deliverd from Topics
type DeliveryPolicy struct {
	// DeliveryGuarantee is used to configure the delivery guarantee of a Topic
	DeliveryGuarantee DeliveryGuarantee
	// Ordered should be set to true if messages should grouped by GroupingKey and
	// be delivered in the order they were published
	Ordered bool
	// GroupingKey is the name of the message attribute used to
	// partition messages into groups with guaranteed ordered
	// delivery.
	// GroupingKey must be set if `Ordered` is true
	GroupingKey string
}

// TopicConfig is used when creating a Topic
type TopicConfig struct {
	// AWS does not support mixing FIFO SNS with standard SQS.
	// Therefore, if one subscription is Ordered/OrderedExactlyOnce,
	// all others must be too.
	DeliveryPolicy *DeliveryPolicy
}
