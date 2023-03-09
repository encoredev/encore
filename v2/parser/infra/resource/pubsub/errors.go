package pubsub

import (
	"encr.dev/pkg/errors"
)

const (
	pubsubNewTopicHelp = "For example `pubsub.NewTopic[MyMessage](\"my-topic\", pubsub.TopicConfig{ DeliveryGuarantee: pubsub.AtLeastOnce })`"

	pubsubNewSubscriptionHelp = "A pubsub subscription must have a unique name per topic and be given a handler function for processing the message. " +
		"The handler for the subscription must be defined in the same service as the call to pubsub.NewSubscription and can be an inline function. " +
		"For example:\n" +
		"\tpubsub.NewSubscription(myTopic, \"subscription-name\", pubsub.SubscriptionConfig[MyMessage]{\n" +
		"\t\tHandler: func(ctx context.Context, event MyMessage) error { return nil },\n" +
		"\t})"
)

var (
	errRange = errors.Range(
		"pubsub",
		"For more information on PubSub, see https://encore.dev/docs/primitives/pubsub",
	)

	errNewTopicArgCount = errRange.Newf(
		"Invalid pubsub.NewTopic call",
		"A call to pubsub.NewTopic requires 2 arguments; the topic name and the config object, got %d arguments.",
		errors.PrependDetails(pubsubNewTopicHelp),
	)

	errInvalidMessageType = errRange.New(
		"Invalid PubSub message type",
		"The message type for a PubSub topic or subscription must be a named struct type.",
		errors.PrependDetails(pubsubNewTopicHelp),
	)

	errOrderingKeyNotExported = errRange.New(
		"Invalid PubSub topic config",
		"The configuration field named \"OrderingKey\" must be a one of the exported fields on the message type.",
		errors.PrependDetails(pubsubNewTopicHelp),
	)

	errNewSubscriptionArgCount = errRange.Newf(
		"Invalid pubsub.NewSubscription call",
		"A call to pubsub.NewSubscription requires 3 arguments; the topic, the subscription name and the config object, got %d arguments.",
		errors.PrependDetails(pubsubNewSubscriptionHelp),
	)

	errSubscriptionTopicNotResource = errRange.New(
		"Invalid call to pubsub.NewSubscription",
		"pubsub.NewSubscription requires the first argument to be a resource of type pubsub.Topic.",
		errors.PrependDetails(pubsubNewSubscriptionHelp),
	)
)
