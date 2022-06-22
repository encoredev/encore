package pubsub

import (
	"context"

	internal "encore.dev/pubsub/internal/types"
)

// SubscriptionConfig is used when creating a subscription
type SubscriptionConfig = internal.SubscriptionConfig

type RetryPolicy = internal.RetryPolicy

// Subscriber is a function reference
// The signature must be `func(context.Context, msg M) error` where M is either the
// message type of the topic or RawMessage
type Subscriber[T any] func(ctx context.Context, msg T) error

type Subscription[T any] interface{}

// DeliveryGuarantee is used to configure the delivery contract for a topic
type DeliveryGuarantee = internal.DeliveryGuarantee

const (
	// AtLeastOnce guarantees that a message for a subscription is delivered to
	// a subscriber at least once
	AtLeastOnce = internal.AtLeastOnce

	// ExactlyOnce guarantees that a message for a subscription is delivered to
	// a subscriber exactly once
	ExactlyOnce = internal.ExactlyOnce
)

// DeliveryPolicy configures how messages are deliverd from Topics
type DeliveryPolicy = internal.DeliveryPolicy

// TopicConfig is used when creating a Topic
type TopicConfig = internal.TopicConfig
