package pubsub

import (
	"encore.dev/pubsub/internal/test"
	"encore.dev/runtime"
)

func GetTestTopicInstance[T any](topic *Topic[T]) any {
	testTopic, ok := topic.topic.(*test.TestTopic[T])
	if !ok {
		panic("testTopic not called with a test topic")
	}

	return testTopic.TestInstance(runtime.CurrentTest())
}
