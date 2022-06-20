package pubsub

import (
	meta "encr.dev/proto/encore/parser/meta/v1"
)

// IsUsed reports whether the application uses pubsub at all.
func IsUsed(md *meta.Data) bool {
	return len(md.PubsubTopics) > 0
}
