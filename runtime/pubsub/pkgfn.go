//go:build encore_app

package pubsub

var Singleton *Manager

func NewTopic[T any](name string, cfg TopicConfig) *Topic[T] {
	return newTopic[T](Singleton, name, cfg)
}
