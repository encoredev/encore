//go:build !encore_app

package pubsub

func newTopic[T any](name string, cfg TopicConfig) *Topic[T] {
	if true {
		panic("only implemented at app runtime")
	}
	return nil
}
