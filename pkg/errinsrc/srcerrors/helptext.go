package srcerrors

import (
	"fmt"
	"strings"
)

func combine(parts ...string) string {
	return strings.Join(parts, "\n\n")
}

const (
	internalErrReportToEncore = "This is a bug in Encore and should not have occurred. Please report this issue to the " +
		"Encore team either on Github at https://github.com/encoredev/encore/issues/new and include this error."

	makeService = "To make this package a count as a service, this package or one of it's parents must have either one " +
		"or more API's declared within them or a PubSub subscription."

	configHelp = "For more information on configuration, see https://encore.dev/docs/develop/config"

	pubsubNewTopicHelp = "For example `pubsub.NewTopic[MyMessage](\"my-topic\", pubsub.TopicConfig{ DeliveryGuarantee: pubsub.AtLeastOnce })`"

	pubsubNewSubscriptionHelp = "A pubsub subscription must have a unique name per topic and be given a handler function for processing the message. " +
		"The handler for the subscription must be defined in the same service as the call to pubsub.NewSubscription and can be an inline function. " +
		"For example:\n" +
		"\tpubsub.NewSubscription(myTopic, \"subscription-name\", pubsub.SubscriptionConfig[MyMessage]{\n" +
		"\t\tHandler: func(ctx context.Context, event MyMessage) error { return nil },\n" +
		"\t})"

	pubsubHelp = "For more information on PubSub, see https://encore.dev/docs/primitives/pubsub"

	metricsHelp = "For more information on metrics, see https://encore.dev/docs/observability/metrics"

	serviceHelp = "For more information on services and how to define them, see https://encore.dev/docs/primitives/services-and-apis"

	authHelp = "For more information on auth handlers and how to define them, see https://encore.dev/docs/develop/auth"
)

func resourceNameHelpKebabCase(resourceName string, paramName string) string {
	return fmt.Sprintf("%s %s's must be defined as string literals, "+
		"be between 1 and 63 characters long, and defined in \"kebab-case\", meaning it must start with a letter, end with a letter "+
		"or number and only contain lower case letters, numbers and dashes.",
		resourceName, paramName,
	)
}

func resourceNameHelpSnakeCase(resourceName string, paramName string) string {
	return fmt.Sprintf("%s %s's must be defined as string literals, "+
		"be between 1 and 63 characters long, and defined in \"snake_case\", meaning it must start with a letter, end with a letter "+
		"or number and only contain lower case letters, numbers and underscores.",
		resourceName, paramName,
	)
}
