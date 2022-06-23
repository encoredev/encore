package parser

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
)

func init() {
	registerResource(
		est.PubSubTopicResource,
		"pubsub topic",
		"https://encore.dev/docs/develop/pubsub",
		"pubsub",
		"encore.dev/pubsub",
	)

	registerResourceCreationParser(
		est.PubSubTopicResource,
		"NewTopic", 1,
		(*parser).parsePubSubTopic,
		locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
	)

	registerResourceUsageParser(
		est.PubSubTopicResource,
		"NewSubscription",
		(*parser).parsePubSubSubscription,
		locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
	)

	registerResourceUsageParser(
		est.PubSubTopicResource,
		"Publish",
		(*parser).parsePubSubPublish,
		locations.AllowedIn(locations.Function).ButNotIn(locations.InitFunction),
	)
}

func (p *parser) parsePubSubTopic(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) < 1 {
		p.errf(callExpr.Pos(), "pubsub.NewTopic requires at least one argument, the topic name given as a string literal. For example `pubsub.NewTopic[MyMessage](\"my-topic\")`")
		return nil
	}

	topicName, ok := litString(callExpr.Args[0])
	if !ok {
		p.errf(callExpr.Args[0].Pos(), "pubsub.NewTopic requires the first argument to be a string literal, was given a %v.", reflect.TypeOf(callExpr.Args[0]))
		return nil
	}
	topicName = strings.TrimSpace(topicName)
	if len(topicName) <= 0 {
		p.errf(callExpr.Args[0].Pos(), "pubsub.NewTopic requires the first argument to be a string literal, was given an empty string.")
		return nil
	}

	// check the topic isn't already declared somewhere else
	for _, topic := range p.pubSubTopics {
		if strings.EqualFold(topic.Name, topicName) {
			p.errf(callExpr.Args[0].Pos(), "Pubsub topic names must be unique, \"%s\" was previously declared in %s/%s: if you wish to reuse the same topic, then you can export the original Topic object from %s and reuse it here.", topic.Name, topic.DeclFile.Pkg.Name, topic.DeclFile.Name, topic.DeclFile.Pkg.Name)
			return nil
		}
	}

	messageType := p.resolveParameter("pubsub message type", file.Pkg, file, getTypeArguments(callExpr.Fun)[0])

	// Record the topic
	topic := &est.PubSubTopic{
		Name:              topicName,
		Doc:               cursor.DocComment(),
		DeliveryGuarantee: est.AtLeastOnce,
		Ordered:           false,
		GroupingField:     "",
		DeclFile:          file,
		MessageType:       messageType,
		IdentAST:          ident,
		Subscribers:       nil,
		Publishers:        nil,
	}
	p.pubSubTopics = append(p.pubSubTopics, topic)

	return topic
}

func (p *parser) parsePubSubSubscription(file *est.File, resource est.Resource, _ *walker.Cursor, callExpr *ast.CallExpr) {
	topic, ok := resource.(*est.PubSubTopic)
	if !ok {
		p.errf(
			callExpr.Fun.Pos(),
			"%s.NewSubscription can only be used on a pubsub topic, was given a %v.",
			resource.Ident().Name, reflect.TypeOf(resource),
		)
		return
	}

	if len(callExpr.Args) < 2 {
		p.errf(
			callExpr.Pos(),
			"%s.NewSubscription requires at least two arguments, the subscription name given as a string literal and the function to consume messages",
			resource.Ident().Name,
		)
		return
	}

	subscriberName, ok := litString(callExpr.Args[0])
	if !ok {
		p.errf(
			callExpr.Args[0].Pos(),
			"%s.NewSubscription requires the first argument to be a string literal, was given a %v.",
			resource.Ident().Name, reflect.TypeOf(callExpr.Args[0]),
		)
		return
	}
	subscriberName = strings.TrimSpace(subscriberName)
	if len(subscriberName) <= 0 {
		p.errf(callExpr.Args[0].Pos(), "%s.NewSubscription requires the first argument to be a string literal, was given an empty string.", resource.Ident().Name)
		return
	}

	// check the subscription isn't already declared somewhere else
	for _, subscriber := range topic.Subscribers {
		if strings.EqualFold(subscriber.Name, subscriberName) {
			p.errf(
				callExpr.Args[0].Pos(),
				"Subscriptions on topics must be unique, \"%s\" was previously declared in %s/%s.",
				subscriber.Name, subscriber.DeclFile.Pkg.Name, subscriber.DeclFile.Name,
			)
			return
		}
	}

	funcDecl, funcFile := p.findFuncFor(
		callExpr.Args[1], file,
		fmt.Sprintf(
			"The function passed as the second argument to `%s.NewSubscription`",
			resource.Ident().Name,
		),
	)
	if funcDecl == nil {
		// The error is reported by p.findFuncFor
		return
	}

	// If the "NewSubscription" function call is not inside a service, then we'll make it a service.
	if file.Pkg.Service == nil {
		p.createService(file.Pkg)
	}

	if funcFile.Pkg.Service == nil {
		p.errf(
			callExpr.Args[1].Pos(),
			"The function passed to `%s.NewSubscription` must be declared in the same service. Currently the function is not declared within a service.",
			resource.Ident().Name,
		)
		return
	}

	if funcFile.Pkg.Service != file.Pkg.Service {
		p.errf(
			callExpr.Args[1].Pos(),
			"The call to `%s.NewSubscription` must be declared in the same service as the function passed in"+
				"as the second argument. The call was made in %s, but the function was declared in %s.",
			resource.Ident().Name, file.Pkg.Service.Name, funcFile.Pkg.Service.Name,
		)
		return
	}

	// Record the subscription
	subscription := &est.PubSubSubscriber{
		Name:     subscriberName,
		CallSite: callExpr,
		Func:     funcDecl,
		FuncFile: funcFile,
		DeclFile: file,
	}
	topic.Subscribers = append(topic.Subscribers, subscription)

	// Record the reference to the topic declaration
	funcFile.References[subscription.CallSite] = &est.Node{
		Type:       est.PubSubSubscriberNode,
		Topic:      topic,
		Subscriber: subscription,
	}
}

func (p *parser) parsePubSubPublish(file *est.File, resource est.Resource, _ *walker.Cursor, callExpr *ast.CallExpr) {
	topic, ok := resource.(*est.PubSubTopic)
	if !ok {
		p.errf(callExpr.Fun.Pos(), "pubsub.Publish can only be used on a pubsub topic, was given a %v.", reflect.TypeOf(resource))
		return
	}

	// Record the publisher
	topic.Publishers = append(topic.Publishers, &est.PubSubPublisher{
		DeclFile: file,
	})

	file.References[callExpr] = &est.Node{
		Type:  est.PubSubPublisherNode,
		Topic: topic,
	}
}
