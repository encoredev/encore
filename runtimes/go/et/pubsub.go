package et

import (
	"encore.dev/pubsub"
)

// Topic returns a TopicHelper for the given topic.
func Topic[T any](topic *pubsub.Topic[T]) TopicHelpers[T] {
	return pubsub.GetTestTopicInstance(topic).(TopicHelpers[T])
}

// TopicHelpers provides functions for interacting with the backing topic implementation
// during unit tests. It is designed to help test code that uses the pubsub.Topic
//
// Note all functions on this TopicHelpers are scoped to the current test
// and will only impact and observe state from the current test
type TopicHelpers[T any] interface {
	// PublishedMessages returns a slice of all messages published during this test on this topic.
	PublishedMessages() []T
}
