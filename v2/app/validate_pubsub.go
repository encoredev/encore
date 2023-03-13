package app

import (
	"encr.dev/pkg/errors"
	"encr.dev/v2/internal/parsectx"
	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/parser/infra/pubsub"
)

func (d *Desc) validatePubSub(pc *parsectx.Context) {
	type topic struct {
		resource *pubsub.Topic
		subs     map[string]*pubsub.Subscription
	}
	topics := make(map[string]topic)
	topicsByBinding := make(map[pkginfo.QualifiedName]string)

	var subs []*pubsub.Subscription

	for _, res := range d.Parse.Resources() {
		switch res := res.(type) {
		case *pubsub.Topic:
			if existing, ok := topics[res.Name]; ok {
				pc.Errs.Add(pubsub.ErrTopicNameNotUnique.
					AtGoNode(existing.resource.AST.Args[0], errors.AsHelp("originally defined here")).
					AtGoNode(res.AST.Args[0], errors.AsError("duplicated here")),
				)
			} else {
				topics[res.Name] = topic{
					resource: res,
					subs:     make(map[string]*pubsub.Subscription),
				}

				for _, bind := range d.Parse.PkgDeclBinds(res) {
					topicsByBinding[bind.QualifiedName()] = res.Name
				}
			}
		case *pubsub.Subscription:
			subs = append(subs, res)
		}
	}

	for _, sub := range subs {
		topicName := topicsByBinding[sub.Topic]
		topic, ok := topics[topicName]
		if !ok {
			pc.Errs.Add(pubsub.ErrSubscriptionTopicNotResource.AtGoNode(sub.AST.Args[0]))
			continue
		}

		if existing, ok := topic.subs[sub.Name]; ok {
			pc.Errs.Add(pubsub.ErrSubscriptionNameNotUnique.
				AtGoNode(existing.AST.Args[1], errors.AsHelp("originally defined here")).
				AtGoNode(sub.AST.Args[1], errors.AsError("duplicated here")),
			)
		} else {
			topic.subs[sub.Name] = sub
		}

	}
}
