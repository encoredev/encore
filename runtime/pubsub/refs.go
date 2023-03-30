//go:build ignore

package pubsub

import "context"

type TopicPerms[T any] interface {
	topicRef() T
}

type Publisher[T any] interface {
	Publish(ctx context.Context, msg T) (id string, err error)
	topicRef() T
}

func TopicRef[P TopicPerms[T], T any](topic *Topic[T]) P {
	return any(topic).(P)
}

func (t *Topic[T]) topicRef() T {
	var zero T
	return zero
}
