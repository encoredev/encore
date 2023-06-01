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

	pubsubTopicUsageHelp = "The topic can only be referenced by calling methods on it, or to pass it to pubsub.NewSubscription or et.Topic."

	pubsubMethodHandlerHelp = "For example `pubsub.MethodHandler(Service.MethodName)` or `pubsub.MethodHandler((*Service).MethodName)`.`"
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

	errInvalidDeliveryGuarantee = errRange.New(
		"Invalid PubSub topic config",
		"The configuration field named \"DeliveryGuarantee\" must be set to pubsub.AtLeastOnce or pubsub.ExactlyOnce.",
	)

	errOrderingKeyNotExported = errRange.New(
		"Invalid PubSub topic config",
		"The configuration field named \"OrderingAttribute\" must be a one of the export attributes on the message type.",
		errors.PrependDetails(pubsubNewTopicHelp),
	)

	errInvalidTopicUsage = errRange.New(
		"Invalid reference to pubsub.Topic",
		"A reference to pubsub.Topic is not permissible here.",
		errors.PrependDetails(pubsubTopicUsageHelp),
	)

	errNewSubscriptionArgCount = errRange.Newf(
		"Invalid pubsub.NewSubscription call",
		"A call to pubsub.NewSubscription requires 3 arguments; the topic, the subscription name and the config object, got %d arguments.",
		errors.PrependDetails(pubsubNewSubscriptionHelp),
	)

	ErrSubscriptionTopicNotResource = errRange.New(
		"Invalid call to pubsub.NewSubscription",
		"pubsub.NewSubscription requires the first argument to be a resource of type pubsub.Topic.",
		errors.PrependDetails(pubsubNewSubscriptionHelp),
	)

	errInvalidAttrPrefix = errRange.New(
		"Invalid attribute prefix",
		"PubSub message attributes must not be prefixed with \"encore\".",
	)

	ErrTopicNameNotUnique = errRange.New(
		"Duplicate PubSub topic name",
		"A PubSub topic name must be unique within a service.",

		errors.PrependDetails("If you wish to reuse the same topic, then you can export the original Topic object import it here."),
	)

	ErrSubscriptionNameNotUnique = errRange.New(
		"Duplicate PubSub subscription on topic",
		"Subscription names on topics must be unique.",
	)

	ErrUnableToIdentifyServicesInvolved = errRange.New(
		"Unable to identify services involved",
		"Unable to identify services involved in the PubSub subscription.",
		errors.MarkAsInternalError(),
	)

	ErrSubscriptionHandlerNotDefinedInSameService = errRange.New(
		"Invalid PubSub subscription handler",
		"The handler for the subscription must be defined in the same service as the call to pubsub.NewSubscription.",
		errors.PrependDetails(pubsubNewSubscriptionHelp),
	)

	errSubscriptionAckDeadlineTooShort = errRange.New(
		"Invalid PubSub subscription config",
		"The ack deadline must be at least 1 second.",
	)

	errSubscriptionMessageRetentionTooShort = errRange.New(
		"Invalid PubSub subscription config",
		"The message retention must be at least 1 minute.",
	)

	errSubscriptionMinRetryBackoffTooShort = errRange.New(
		"Invalid PubSub subscription config",
		"The min backoff for retries must be at least 1 second.",
	)

	errSubscriptionMaxRetryBackoffTooShort = errRange.New(
		"Invalid PubSub subscription config",
		"The max backoff for retries must be at least 1 second.",
	)

	errSubscriptionMaxRetriesTooSmall = errRange.New(
		"Invalid PubSub subscription config",
		"The max number of retries must be a positive number or the constants `pubsub.InfiniteRetries` or `pubsub.NoRetries`.",
	)

	errTopicRefNoTypeArgs = errRange.New(
		"Invalid call to pubsub.TopicRef",
		"A type argument indicating the requested permissions must be provided.",
	)

	errTopicRefInvalidPerms = errRange.New(
		"Unrecognized permissions in call to pubsub.TopicRef",
		"The only supported permission is currently pubsub.Publisher[MyMessage].",
	)

	ErrTopicRefOutsideService = errRange.New(
		"Call to pubsub.TopicRef outside service",
		"pubsub.TopicRef can only be called from within a service.",
	)

	ErrInvalidMethodHandler = errRange.New(
		"Invalid call to pubsub.MethodHandler",
		"pubsub.MethodHandler requires the first argument to be a reference to a method on a service struct.",
		errors.PrependDetails(pubsubMethodHandlerHelp),
	)

	ErrMethodHandlerTypeNotServiceStruct = errRange.New(
		"Invalid call to pubsub.MethodHandler",
		"pubsub.MethodHandler can only reference methods that are defined on service structs.",
		errors.PrependDetails(pubsubMethodHandlerHelp),
	)

	ErrMethodHandlerDifferentPackage = errRange.New(
		"Invalid call to pubsub.MethodHandler",
		"pubsub.MethodHandler can only reference the service struct defined in the same package as the subscription.",
		errors.PrependDetails(pubsubMethodHandlerHelp),
	)
)
