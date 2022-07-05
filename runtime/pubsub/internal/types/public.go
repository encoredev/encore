package types

import (
	"time"
)

// RetryPolicy defines how a subscription should handle retries
// after errors either delivering the message or processing the message.
//
// The values given to this structure are parsed at compile time, such that
// the correct Cloud resources can be provisioned to support the queue.
//
// As such the values given here may be clamped to the supported values by
// the target cloud. (i.e. min/max values brought within the supported range
// by the target cloud).
type RetryPolicy struct {
	// The minimum time to wait between retries. Defaults to 10 seconds.
	MinRetryDelay time.Duration

	// The maximum time to wait between retries. Defaults to 10 minutes.
	MaxRetryDelay time.Duration

	// MaxRetries is used to control deadletter queuing logic, when:
	//   n == 0: A default value of 100 retries will be used
	//   n > 0:  Encore will forward a message to a dead letter queue after n retries
	//   n == pubsub.InfiniteRetries: Messages will not be forwarded to the dead letter queue by the Encore framework
	MaxRetries int
}

const (
	// NoRetries is used to control deadletter queuing logic, when set as the MaxRetires within the RetryPolicy
	// it will attempt to immediately forward a message to the dead letter queue if the subscription Handler
	// returns any error or panics.
	//
	// Note: With some cloud providers, having no retries may not be supported, in which case the minimum number of
	// retries permitted by the provider will be used.
	NoRetries = -2

	// InfiniteRetries is used to control deadletter queuing logic, when set as the MaxRetires within the RetryPolicy
	// it will attempt to always retry a message without ever sending it to the dead letter queue.
	//
	// Note: With some cloud providers, infinite retries may not be supported, in which case the maximum number of
	// retries permitted by the provider will be used.
	InfiniteRetries = -1
)

// DeliveryGuarantee is used to configure the delivery contract for a topic
type DeliveryGuarantee int

const (
	// AtLeastOnce guarantees that a message for a subscription is delivered to
	// a consumer at least once. This is supported by all cloud providers.
	AtLeastOnce DeliveryGuarantee = iota + 1

	// ExactlyOnce guarantees that a message for a subscription is delivered to
	// a consumer exactly once
	// ExactlyOnce // - ExactlyOnce is currently not supported.
)

// TopicConfig is used when creating a Topic
type TopicConfig struct {
	// DeliveryGuarantee is used to configure the delivery guarantee of a Topic
	//
	// This field is required.
	DeliveryGuarantee DeliveryGuarantee

	// OrderingKey is the name of the message attribute used to group
	// messages and delivery messages with the same OrderingKey value
	// in the order they were published.
	//
	// If OrderingKey is not set, messages can be delivered in any order.
	// OrderingKey string - OrderingKey is currently not supported.
}
