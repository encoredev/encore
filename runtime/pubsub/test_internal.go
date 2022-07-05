package pubsub

import (
	"encore.dev/pubsub/internal/test"
	"encore.dev/runtime"
)

// GetTestTopicInstance is an internal API for Encore. This function should
// never be directly called as it is considered an unstable API and Encore
// can change it at any time
func GetTestTopicInstance[T any](topic *Topic[T]) any {
	testTopic, ok := topic.topic.(*test.TestTopic[T])
	if !ok {
		panic("testTopic not called with a test topic")
	}

	return testTopic.TestInstance(runtime.CurrentTest())
}
