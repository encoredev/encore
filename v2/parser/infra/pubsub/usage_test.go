package pubsub_test

import (
	"testing"

	"encr.dev/v2/parser/infra/pubsub"
	"encr.dev/v2/parser/resource/usage"
	"encr.dev/v2/parser/resource/usage/usagetest"
)

func TestResolveTopicUsage(t *testing.T) {
	tests := []usagetest.Case{
		{
			Name: "none",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

`,
			Want: []usage.Usage{},
		},
		{
			Name: "publish",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

func Foo() { topic.Publish(context.Background(), Msg{}) }

`,
			Want: []usage.Usage{&pubsub.PublishUsage{}},
		},
		{
			Name: "ref",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

var ref = pubsub.TopicRef[pubsub.Publisher[Msg]](topic)
`,
			Want: []usage.Usage{&pubsub.RefUsage{
				Perms: []pubsub.Perm{pubsub.PublishPerm},
			}},
		},
		{
			Name: "custom_ref_alias",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

type MyRef = pubsub.Publisher[Msg]

var ref = pubsub.TopicRef[MyRef](topic)
`,
			Want: []usage.Usage{&pubsub.RefUsage{
				Perms: []pubsub.Perm{pubsub.PublishPerm},
			}},
		},
		{
			Name: "custom_ref_interface",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

type MyRef interface { pubsub.Publisher[Msg] }

var ref = pubsub.TopicRef[MyRef](topic)
`,
			Want: []usage.Usage{&pubsub.RefUsage{
				Perms: []pubsub.Perm{pubsub.PublishPerm},
			}},
		},
		{
			Name: "generic_custom_ref_interface",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

type MyRef[T any] interface { pubsub.Publisher[T] }

var ref = pubsub.TopicRef[MyRef[Msg]](topic)
`,
			Want: []usage.Usage{&pubsub.RefUsage{
				Perms: []pubsub.Perm{pubsub.PublishPerm},
			}},
		},
		{
			Name: "invalid_ref",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

type MyRef interface { pubsub.Publisher[Msg]; ~int | string; Publish() int }

var ref = pubsub.TopicRef[MyRef](topic)
`,
			WantErrs: []string{"Unrecognized permissions in call to pubsub.TopicRef"},
		},
		{
			Name: "invalid_ref_2",
			Code: `
type Msg struct{}

var topic = pubsub.NewTopic[Msg]("topic", pubsub.TopicConfig{DeliveryGuarantee: pubsub.AtLeastOnce})

var ref = pubsub.TopicRef[string](topic)
`,
			WantErrs: []string{"Unrecognized permissions in call to pubsub.TopicRef"},
		},
	}

	usagetest.Run(t, []string{"encore.dev/pubsub"}, tests)
}
