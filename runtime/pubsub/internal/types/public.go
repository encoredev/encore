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
	MinBackoff time.Duration

	// The maximum time to wait between retries. Defaults to 10 minutes.
	MaxBackoff time.Duration

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
	// a consumer at least once.
	//
	// On AWS and GCP there is no limit to the throughput for a topic.
	AtLeastOnce DeliveryGuarantee = iota + 1

	// ExactlyOnce guarantees that a message for a subscription is delivered to
	// a consumer exactly once, to the best of the system's ability.
	//
	// However, there are edge cases when a message might be redelivered.
	// For example, if a networking issue causes the acknowledgement of success
	// processing the message to be lost before the cloud provider receives it.
	//
	// It is also important to note that the ExactlyOnce delivery guarantee only
	// applies to the delivery of the message to the consumer, and not to the
	// original publishing of the message, such that if a message is published twice,
	// such as due to an retry within the application logic, it will be delivered twice.
	// (i.e. ExactlyOnce delivery does not imply message deduplication on publish)
	//
	// As such it's recommended that the subscription handler function is idempotent
	// and is able to handle duplicate messages.
	//
	// Subscriptions attached to ExactlyOnce topics have higher message delivery latency compared to AtLeastOnce.
	//
	// By using ExactlyOnce semantics on a topic, the throughput will be limited depending on the cloud provider:
	//
	// - AWS: 300 messages per second for the topic (see [AWS SQS Quotas]).
	// - GCP: At least 3,000 messages per second across all topics in the region
	// 		  (can be higher on the region see [GCP PubSub Quotas]).
	//
	// [AWS SQS Quotas]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
	// [GCP PubSub Quotas]: https://cloud.google.com/pubsub/quotas#quotas
	ExactlyOnce
)

// TopicConfig is used when creating a Topic
type TopicConfig struct {
	// DeliveryGuarantee is used to configure the delivery guarantee of a Topic
	//
	// This field is required.
	DeliveryGuarantee DeliveryGuarantee

	// OrderingAttribute is the message attribute to use as a ordering key for
	// messages and delivery will ensure that messages with the same value will
	// be delivered in the order they where published.
	//
	// If OrderingAttribute is not set, messages can be delivered in any order.
	//
	// It is important to note, that in the case of an error being returned by a
	// subscription handler, the message will be retried before any subsequent
	// messages for that ordering key are delivered. This means depending on the
	// retry configuration, a large backlog of messages for a given ordering key
	// may build up. When using OrderingAttribute, it is recommended to use reason
	// about your failure modes and set the retry configuration appropriately.
	//
	// Once the maximum number of retries has been reached, the message will be
	// forwarded to the dead letter queue, and the next message for that ordering
	// key will be delivered.
	//
	// To create attributes on a message, use the `pubsub-attr` struct tag:
	//
	//	type UserEvent struct {
	//		UserID string `pubsub-attr:"user-id"`
	//		Action string
	//	}
	//
	//  var topic = pubsub.NewTopic[UserEvent]("user-events", pubsub.TopicConfig{
	// 		DeliveryGuarantee: pubsub.AtLeastOnce,
	//		OrderingAttribute: "user-id", // Messages with the same user-id will be delivered in the order they where published
	//	})
	//
	//  topic.Publish(ctx, &UserEvent{UserID: "1", Action: "login"})  // This message will be delivered before the logout
	//  topic.Publish(ctx, &UserEvent{UserID: "2", Action: "login"})  // This could be delivered at any time because it has a different user id
	//  topic.Publish(ctx, &UserEvent{UserID: "1", Action: "logout"}) // This message will be delivered after the first message
	//
	// By using OrderingAttribute, the throughput will be limited depending on the cloud provider:
	//
	// - AWS: 300 messages per second for the topic (see [AWS SQS Quotas]).
	// - GCP: 1MB/s for each ordering key (see [GCP PubSub Quotas]).
	//
	// Note: OrderingAttribute currently has no effect during local development.
	//
	// [AWS SQS Quotas]: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
	// [GCP PubSub Quotas]: https://cloud.google.com/pubsub/quotas#resource_limits
	OrderingAttribute string
}
