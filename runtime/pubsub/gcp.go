//go:build !encore_local

package pubsub

// NewTopic is used to declare a Topic. Encore will use static
// analysis to identify Topics and automatically provision them
// for you.
func NewTopic[T any](name string, cfg *TopicConfig) Topic[T] {
	return nil
}
