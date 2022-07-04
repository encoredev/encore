package pubsub

import (
	"encore.dev/pubsub/internal/types"
)

type SubscriptionConfig = types.SubscriptionConfig

type RetryPolicy = types.RetryPolicy

type DeliveryGuarantee = types.DeliveryGuarantee

const (
	AtLeastOnce = types.AtLeastOnce
)

type TopicConfig = types.TopicConfig
