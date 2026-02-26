//go:build encore_app

package pubsub

func newTopic[T any](name string, cfg TopicConfig) *Topic[T] {
	return newTopic2[T](Singleton, name, cfg)
}
