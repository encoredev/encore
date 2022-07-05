package parser

import (
	"go/ast"
	"reflect"
	"strings"

	"github.com/fatih/structtag"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	schema "encr.dev/proto/encore/parser/schema/v1"
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
		"Publish",
		(*parser).parsePubSubPublish,
		locations.AllowedIn(locations.Function).ButNotIn(locations.InitFunction),
	)

	registerResource(
		est.PubSubSubscriptionResource,
		"pubsub subscription",
		"https://encore.dev/docs/develop/pubsub",
		"pubsub",
		"encore.dev/pubsub",
		est.PubSubTopicResource,
	)

	// NewSubscription can be called with 0 or 1 type parameter, so we register against both
	for i := 0; i <= 1; i++ {
		registerResourceCreationParser(
			est.PubSubSubscriptionResource,
			"NewSubscription", i,
			(*parser).parsePubSubSubscription,
			locations.AllowedIn(locations.Variable).ButNotIn(locations.Function),
		)
	}

	registerStructTagParser(
		"pubsub-attr",
		(*parser).parsePubSubAttr,
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

func (p *parser) parsePubSubSubscription(file *est.File, cursor *walker.Cursor, ident *ast.Ident, callExpr *ast.CallExpr) est.Resource {
	if len(callExpr.Args) < 3 {
		p.err(
			callExpr.Pos(),
			"pubsub.NewSubscription requires at least three arguments, the topic, the subscription name given as a string literal and the function to consume messages",
		)
		return nil
	}

	resource := p.resourceFor(file, callExpr.Args[0])
	if resource == nil {
		p.errf(callExpr.Args[0].Pos(), "pubsub.NewSubscription requires the first argument to reference to pubsub topic, was given %v.", prettyPrint(callExpr.Args[0]))
		return nil
	}
	topic, ok := resource.(*est.PubSubTopic)
	if !ok {
		p.errf(
			callExpr.Fun.Pos(),
			"pubsub.NewSubscription can only be used on a pubsub topic, was given a %v.",
			reflect.TypeOf(resource),
		)
		return nil
	}

	subscriberName, ok := litString(callExpr.Args[1])
	if !ok {
		p.errf(
			callExpr.Args[1].Pos(),
			"pubsub.NewSubscription requires the first argument to be a string literal, was given a %v.",
			reflect.TypeOf(callExpr.Args[1]),
		)
		return nil
	}
	subscriberName = strings.TrimSpace(subscriberName)
	if len(subscriberName) <= 0 {
		p.err(callExpr.Args[1].Pos(), "pubsub.NewSubscription requires the first argument to be a string literal, was given an empty string.")
		return nil
	}

	// check the subscription isn't already declared somewhere else
	for _, subscriber := range topic.Subscribers {
		if strings.EqualFold(subscriber.Name, subscriberName) {
			p.errf(
				callExpr.Args[1].Pos(),
				"Subscriptions on topics must be unique, \"%s\" was previously declared in %s/%s.",
				subscriber.Name, subscriber.DeclFile.Pkg.Name, subscriber.DeclFile.Name,
			)
			return nil
		}
	}

	funcDecl, funcFile := p.findFuncFor(
		callExpr.Args[2], file,
		"The function passed as the second argument to `pubsub.NewSubscription`",
	)
	if funcDecl == nil {
		// The error is reported by p.findFuncFor
		return nil
	}

	// If the "NewSubscription" function call is not inside a service, then we'll make it a service.
	if file.Pkg.Service == nil {
		p.createService(file.Pkg)
	}

	if funcFile.Pkg.Service == nil {
		p.err(
			callExpr.Args[1].Pos(),
			"The function passed to `pubsub.NewSubscription` must be declared in the same service. Currently the function is not declared within a service.",
		)
		return nil
	}

	if funcFile.Pkg.Service != file.Pkg.Service {
		p.errf(
			callExpr.Args[1].Pos(),
			"The call to `pubsub.NewSubscription` must be declared in the same service as the function passed in"+
				"as the second argument. The call was made in %s, but the function was declared in %s.",
			file.Pkg.Service.Name, funcFile.Pkg.Service.Name,
		)
		return nil
	}

	// Record the subscription
	subscription := &est.PubSubSubscriber{
		Name:     subscriberName,
		Topic:    topic,
		CallSite: callExpr,
		Func:     funcDecl,
		FuncFile: funcFile,
		DeclFile: file,
		IdentAST: ident,
	}
	topic.Subscribers = append(topic.Subscribers, subscription)
	return subscription
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
		Type: est.PubSubPublisherNode,
		Res:  topic,
	}
}

func (p *parser) parsePubSubAttr(rawTag *ast.BasicLit, parsedTag *structtag.Tag, structType *schema.Struct, fieldName string, fieldType *schema.Type) {
	if strings.HasPrefix(strings.ToLower(parsedTag.Name), "encore") {
		p.errf(rawTag.Pos(), "Pubsub attribute tags must not start with \"encore\". The field %s currently has an attribute tag of \"%s\".", fieldName, parsedTag.Name)
	}
}
