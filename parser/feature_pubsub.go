package parser

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"
	"time"

	"github.com/fatih/structtag"

	"encr.dev/parser/est"
	"encr.dev/parser/internal/locations"
	"encr.dev/parser/internal/walker"
	"encr.dev/pkg/errinsrc/srcerrors"
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
		p.errInSrc(srcerrors.PubSubNewTopicInvalidArgCount(p.fset, callExpr))
		return nil
	}

	topicName := p.parseResourceName("pubsub.NewTopic", "topic name", callExpr.Args[0], kebabName, "")
	if topicName == "" {
		// we already reported the error inside parseResourceName
		return nil
	}

	// check the topic isn't already declared somewhere else
	for _, topic := range p.pubSubTopics {
		if strings.EqualFold(topic.Name, topicName) {
			p.errInSrc(srcerrors.PubSubTopicNameNotUnique(p.fset, topic.NameAST, callExpr.Args[0]))
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
			p.errInSrc(srcerrors.PubSubTopicConfigNotConstant(p.fset, fieldName, expr))
		}
		return nil
	}

	if !cfg.IsSet("DeliveryGuarantee") {
		p.errInSrc(srcerrors.PubSubTopicConfigMissingField(p.fset, "DeliveryGuarantee", callExpr.Args[1]))
		return nil
	}
	deliveryGuarantee := cfg.Int64("DeliveryGuarantee", 0)
	if deliveryGuarantee != 1 && deliveryGuarantee != 2 {
		p.errInSrc(srcerrors.PubSubTopicConfigInvalidField(p.fset, "DeliveryGuarantee", "`pubsub.AtLeastOnce`", cfg.Expr("DeliveryGuarantee")))
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
					p.errInSrc(srcerrors.PubSubOrderingKeyMustBeExported(p.fset, cfg.Expr("OrderingKey")))
				}
			}
		} else {
			p.errInSrc(srcerrors.PubSubOrderingKeyNotStringLiteral(p.fset, cfg.Expr("OrderingKey")))
		}
	}

	// Record the topic
	topic := &est.PubSubTopic{
		Name:              topicName,
		NameAST:           callExpr.Args[0],
		Doc:               cursor.DocComment(),
		DeliveryGuarantee: est.PubSubGuarantee(deliveryGuarantee - 1), // The runtime is 1 indexed
		OrderingKey:       orderingKey,
		DeclFile:          file,
		DeclCall:          callExpr,
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
		p.errInSrc(srcerrors.PubSubSubscriptionArguments(p.fset, callExpr))
		return nil
	}

	resource := p.resourceFor(file, callExpr.Args[0])
	if resource == nil {
		p.errInSrc(srcerrors.PubSubSubscriptionTopicNotResource(p.fset, callExpr.Args[0], ""))
		return nil
	}
	topic, ok := resource.(*est.PubSubTopic)
	if !ok {
		p.errInSrc(srcerrors.PubSubSubscriptionTopicNotResource(p.fset, callExpr.Args[0], fmt.Sprintf("got a %T", reflect.TypeOf(resource))))
		return nil
	}

	subscriberName := p.parseResourceName("pubsub.NewSubscription", "subscription name", callExpr.Args[1], kebabName, "")
	if subscriberName == "" {
		// we already reported the error inside parseResourceName
		return nil
	}

	// check the subscription isn't already declared somewhere else
	for _, subscriber := range topic.Subscribers {
		if strings.EqualFold(subscriber.Name, subscriberName) {
			p.errInSrc(srcerrors.PubSubSubscriptionNameNotUnique(p.fset, subscriber.NameAST, callExpr.Args[1]))
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
			p.errInSrc(srcerrors.PubSubSubscriptionConfigNotConstant(p.fset, fieldName, expr))
			ok = false
		}
	}
	if !ok {
		return nil
	}

	handler := cfg.Expr("Handler")
	if handler == nil {
		p.errInSrc(srcerrors.PubSubSubscriptionRequiresHandler(p.fset, callExpr.Args[2]))
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
		p.errInSrc(srcerrors.PubSubSubscriptionHandlerNotInService(p.fset, handler, funcDecl))
		return nil
	}

	if funcFile.Pkg.Service != file.Pkg.Service {
		p.errInSrc(srcerrors.PubSubSubscriptionHandlerNotInService(p.fset, handler, funcDecl))
		return nil
	}

	// Verify other configuration
	ackDeadline := time.Duration(cfg.Int64("AckDeadline", int64(30*time.Second)))
	if ackDeadline < 1*time.Second {
		p.errInSrc(srcerrors.PubSubSubscriptionInvalidField(p.fset, "AckDeadline", "at least 1 second", cfg.Expr("AckDeadline"), ackDeadline.String()))
	}

	messageRetention := time.Duration(cfg.Int64("MessageRetention", int64(7*24*time.Hour)))
	if messageRetention < 1*time.Minute {
		p.errInSrc(srcerrors.PubSubSubscriptionInvalidField(p.fset, "MessageRetention", "at least 1 minute", cfg.Expr("MessageRetention"), messageRetention.String()))
	}

	minRetryBackoff := time.Duration(cfg.Int64("RetryPolicy.MinBackoff", int64(10*time.Second)))
	if minRetryBackoff < 1*time.Second {
		p.errInSrc(srcerrors.PubSubSubscriptionInvalidField(p.fset, "RetryPolicy.MinBackoff", "at least 1 second", cfg.Expr("RetryPolicy.MinBackoff"), minRetryBackoff.String()))
	}

	maxRetryBackoff := time.Duration(cfg.Int64("RetryPolicy.MaxBackoff", int64(10*time.Minute)))
	if maxRetryBackoff < 1*time.Second {
		p.errInSrc(srcerrors.PubSubSubscriptionInvalidField(p.fset, "RetryPolicy.MaxBackoff", "at least 1 second", cfg.Expr("RetryPolicy.MaxBackoff"), maxRetryBackoff.String()))
	}

	maxRetries := cfg.Int64("RetryPolicy.MaxRetries", 100)
	if maxRetries < -2 {
		p.errInSrc(srcerrors.PubSubSubscriptionInvalidField(p.fset, "RetryPolicy.MaxRetries", "a positive number or the constants `pubsub.InfiniteRetries` or `pubsub.NoRetries`", cfg.Expr("RetryPolicy.MaxRetries"), fmt.Sprintf("%d", maxRetries)))
	}

	// Record the subscription
	subscription := &est.PubSubSubscriber{
		Name:             subscriberName,
		NameAST:          callExpr.Args[1],
		Topic:            topic,
		CallSite:         callExpr,
		Func:             funcDecl,
		FuncFile:         funcFile,
		DeclFile:         file,
		DeclCall:         callExpr,
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

func (p *parser) parsePubSubPublish(file *est.File, resource est.Resource, c *walker.Cursor, callExpr *ast.CallExpr) {
	topic, ok := resource.(*est.PubSubTopic)
	if !ok {
		// This is an internal error, so we panic rather than report an error
		panic(fmt.Sprintf("expected a PubSubTopic, got %T", resource))
		return
	}

	publisher := &est.PubSubPublisher{
		DeclFile: file,
	}

	if file.Pkg.Service == nil {
		middleware := p.findContainingMiddlewareDefinition(c)
		if middleware == nil || !middleware.Global {
			p.errInSrc(srcerrors.PubSubPublishInvalidLocation(p.fset, callExpr))
		}

		publisher.GlobalMiddleware = middleware
	} else {
		publisher.Service = file.Pkg.Service
	}

	// Record the publisher
	topic.Publishers = append(topic.Publishers, publisher)

	file.References[callExpr] = &est.Node{
		Type: est.PubSubPublisherNode,
		Res:  topic,
	}
}

func (p *parser) parsePubSubAttr(rawTag *ast.BasicLit, parsedTag *structtag.Tag, structType *schema.Struct, fieldName string, fieldType *schema.Type) {
	if strings.HasPrefix(strings.ToLower(parsedTag.Name), "encore") {
		p.errInSrc(srcerrors.PubSubAttrInvalidTag(p.fset, rawTag, parsedTag.Name))
	}
}
