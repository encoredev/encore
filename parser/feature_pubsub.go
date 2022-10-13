package parser

import (
	"go/ast"
	"reflect"
	"strings"
	"time"

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
	if len(callExpr.Args) != 2 {
		p.errf(callExpr.Pos(), "pubsub.NewTopic requires at least one argument, the topic name given as a string literal. For example `pubsub.NewTopic[MyMessage](\"my-topic\")`")
		return nil
	}

	topicName := p.parseResourceName("pubsub.NewTopic", "topic name", callExpr.Args[0])
	if topicName == "" {
		// we already reported the error inside parseResourceName
		return nil
	}

	// check the topic isn't already declared somewhere else
	for _, topic := range p.pubSubTopics {
		if strings.EqualFold(topic.Name, topicName) {
			p.errf(callExpr.Args[0].Pos(), "Pubsub topic names must be unique, \"%s\" was previously declared in %s/%s: if you wish to reuse the same topic, then you can export the original Topic object from %s and reuse it here.", topic.Name, topic.DeclFile.Pkg.Name, topic.DeclFile.Name, topic.DeclFile.Pkg.Name)
			return nil
		}
	}

	messageType := p.resolveParameter("pubsub message type", file.Pkg, file, getTypeArguments(callExpr.Fun)[0], true)

	// Parse the literal struct representing the subscription configuration
	// so we can extract the reference to the handler function
	cfg, ok := p.parseStructLit(file, "pubsub.TopicConfig", callExpr.Args[1])
	if !ok {
		return nil
	}
	if !cfg.FullyConstant() {
		for fieldName, expr := range cfg.DynamicFields() {
			p.errf(expr.Pos(), "All values in pubsub.TopicConfig must be a constant, however %s was not a constant, got %s", fieldName, prettyPrint(expr))
		}
		return nil
	}

	if !cfg.IsSet("DeliveryGuarantee") {
		p.errf(cfg.Pos("DeliveryGuarantee"), "pubsub.NewTopic requires the configuration field named \"DeliveryGuarantee\" to be explicitly set.")
		return nil
	}
	deliveryGuarantee := cfg.Int64("DeliveryGuarantee", 0)
	if deliveryGuarantee != 1 {
		p.errf(cfg.Pos("DeliveryGuarantee"), "pubsub.NewTopic requires the configuration field named \"DeliveryGuarantee\" to a valid value such as \"pubsub.AtLeastOnce\".")
		return nil
	}

	// Get the ordering key
	orderingKey := ""
	if cfg.IsSet("OrderingKey") {
		str := cfg.Str("OrderingKey", "")
		if str != "" {
			orderingKey = str

			str := messageType.Type.GetStruct()
			if str != nil {
				found := false
				for _, field := range str.Fields {
					if field.Name == orderingKey {
						found = true
						break
					}
				}

				if !found || !ast.IsExported(orderingKey) {
					p.errf(cfg.Pos("OrderingKey"), "pubsub.NewTopic requires the configuration field named \"OrderingKey\" to be a one of the exported fields on the message type.")
				}
			}
		} else {
			p.errf(cfg.Pos("OrderingKey"), "pubsub.NewTopic requires the configuration field named \"OrderingKey\" to either not be set, or be set to a non empty string referencing the field in the message type you want to order messages by.")
		}
	}

	// Record the topic
	topic := &est.PubSubTopic{
		Name:              topicName,
		Doc:               cursor.DocComment(),
		DeliveryGuarantee: est.AtLeastOnce,
		OrderingKey:       orderingKey,
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
	if len(callExpr.Args) != 3 {
		p.err(
			callExpr.Pos(),
			"pubsub.NewSubscription requires three arguments, the topic, the subscription name given as a string literal and the subscription configuration",
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

	subscriberName := p.parseResourceName("pubsub.NewSubscription", "subscription name", callExpr.Args[1])
	if subscriberName == "" {
		// we already reported the error inside parseResourceName
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

	// Parse the literal struct representing the subscription configuration
	// so we can extract the reference to the handler function
	cfg, ok := p.parseStructLit(file, "pubsub.SubscriptionConfig", callExpr.Args[2])
	if !ok {
		return nil
	}

	// Check everything apart from Handler is constant
	ok = true
	for fieldName, expr := range cfg.DynamicFields() {
		if fieldName != "Handler" {
			p.errf(expr.Pos(), "All values in pubsub.SubscriptionConfig must be a constant, however %s was not a constant, got %s", fieldName, prettyPrint(expr))
			ok = false
		}
	}
	if !ok {
		return nil
	}

	handler := cfg.Expr("Handler")
	if handler == nil {
		p.errf(callExpr.Args[2].Pos(), "pubsub.NewSubscription requires the configuration field named \"Handler\" to populated with the subscription handler function.")
		return nil
	}
	p.validRPCReferences[handler] = true

	funcDecl, funcFile := p.findFuncFor(
		handler, file,
		"The function passed as the Handler argument to `pubsub.SubscriptionConfig`",
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
			"The function passed to `pubsub.NewSubscription` must be declared in the same service. Currently the handler is not declared within a service.",
		)
		return nil
	}

	if funcFile.Pkg.Service != file.Pkg.Service {
		p.errf(
			callExpr.Args[1].Pos(),
			"The call to `pubsub.NewSubscription` must be declared in the same service as the handler passed in"+
				". The call was made in %s, but the handler function was declared in %s.",
			file.Pkg.Service.Name, funcFile.Pkg.Service.Name,
		)
		return nil
	}

	// Verify other configuration
	ackDeadline := time.Duration(cfg.Int64("AckDeadline", int64(30*time.Second)))
	if ackDeadline < 1*time.Second {
		p.errf(cfg.Pos("AckDeadline"), "AckDeadline must be at least 1 second, was %s", ackDeadline)
	}

	messageRetention := time.Duration(cfg.Int64("MessageRetention", int64(7*24*time.Hour)))
	if messageRetention < 1*time.Minute {
		p.errf(cfg.Pos("MessageRetention"), "MessageRetention must be at least 1 minute, was %s", messageRetention)
	}

	minRetryBackoff := time.Duration(cfg.Int64("RetryPolicy.MinBackoff", int64(10*time.Second)))
	if minRetryBackoff < 1*time.Second {
		p.errf(cfg.Pos("RetryPolicy.MinBackoff"), "RetryPolicy.MinBackoff must be at least 1 second, was %s", minRetryBackoff)
	}

	maxRetryBackoff := time.Duration(cfg.Int64("RetryPolicy.MaxBackoff", int64(10*time.Minute)))
	if maxRetryBackoff < 1*time.Second {
		p.errf(cfg.Pos("RetryPolicy.MaxBackoff"), "RetryPolicy.MaxBackoff must be at least 1 second, was %s", minRetryBackoff)
	}

	maxRetries := cfg.Int64("RetryPolicy.MaxRetries", 100)
	if maxRetries < -2 {
		p.errf(cfg.Pos("RetryPolicy.MaxRetries"), "RetryPolicy.MaxRetries must be a positive number or the constants `pubsub.InfiniteRetries` or `pubsub.NoRetries`, was %d", maxRetries)
	}

	// Record the subscription
	subscription := &est.PubSubSubscriber{
		Name:             subscriberName,
		Topic:            topic,
		CallSite:         callExpr,
		Func:             funcDecl,
		FuncFile:         funcFile,
		DeclFile:         file,
		IdentAST:         ident,
		AckDeadline:      ackDeadline,
		MessageRetention: messageRetention,
		MinRetryBackoff:  minRetryBackoff,
		MaxRetryBackoff:  maxRetryBackoff,
		MaxRetries:       maxRetries,
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
