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
	// Handler is the function which will be called to process a message
	// sent on the topic.
	//
	// To reference a method on an [Encore service struct]
	// you can use the [MethodHandler] function. For example:
	//
	//	Handler: pubsub.MethodHandler((*MyService).MyMethod)
	//
	// It is important for the Handler function to block and not return
	// until all processing relating to the message has been completed.
	//
	// When the handler returns a nil error the message will be
	// acknowledged (acked) from the topic, and should not be redelivered.
	//
	// When this function returns a non-nil error the message will be
	// negatively acknowledged (nacked), which will cause a redelivery
	// attempt to be made (unless the retry policy's MaxRetries has been reached).
	//
	// The ctx passed to the handler will be cancelled when
	// the AckDeadline passes.
	//
	// This field is required.
	//
	// [Encore service struct]: https://encore.dev/docs/primitives/services-and-apis/service-structs
	Handler func(ctx context.Context, msg T) error

	// MaxConcurrency is the maximum number of messages which will be processed
	// simultaneously per instance of the service for this subscription.
	//
	// Note that this is per instance of the service, so if your service has
	// scaled to 10 instances and this is set to 10, then 100 messages could be
	// processed simultaneously.
	//
	// If the value is negative, then there will be no limit on the number
	// of messages processed simultaneously.
	//
	// Note: This is not supported by all cloud providers; specifically on GCP
	// when using Cloud Run instances on a topic with at-least-once delivery, the
	// subscription will be configured as a Push Subscription and will have an adaptive
	// concurrency. See [GCP Push Delivery Rate].
	//
	// This setting also has no effect on Encore Cloud environments.
	//
	// If not set, it uses a reasonable default based on the cloud provider.
	//
	// [GCP Push Delivery Rate]: https://cloud.google.com/pubsub/docs/push#push_delivery_rate
	MaxConcurrency int

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
