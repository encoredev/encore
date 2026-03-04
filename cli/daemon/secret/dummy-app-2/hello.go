package hello

import "encore.dev/pubsub"

var Topic = pubsub.NewTopic[*string]("topic", pubsub.TopicConfig{})
