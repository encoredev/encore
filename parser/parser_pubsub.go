package parser

import (
	"go/ast"
	"reflect"
	"strings"

	"encr.dev/parser/est"
	"encr.dev/pkg/identifiers"
)

const pubsubPackage = "encore.dev/pubsub"

func init() {
	defaultTrackedPackages[pubsubPackage] = "pubsub"
	resourceRegistry[pubsubPackage] = map[resourceInitFuncIdent]*resourceParser{
		"NewTopic": {
			ResourceName: "pubsub topic",
			CreationFunc: "pubsub.NewTopic",
			DocsPage:     "https://encore.dev/docs/develop/pubsub",
			Parse:        (*parser).parsePubSubTopic,
		},
	}
}

func (p *parser) parsePubSubTopic(file *est.File, doc string, valueSpec *ast.ValueSpec, callExpr *ast.CallExpr) {
	if len(callExpr.Args) != 1 {
		p.errf(valueSpec.Pos(), "pubsub.NewTopic requires exactly one argument, the topic name given as a string literal. For example `pubsub.NewTopic[MyMessage](\"my-topic\")`")
		return
	}

	topicName, ok := litString(callExpr.Args[0])
	if !ok {
		p.errf(callExpr.Args[0].Pos(), "pubsub.NewTopic requires the first argument to be a string literal, was given a %v.", reflect.TypeOf(callExpr.Args[0]))
		return
	}
	topicName = normaliseTopicName(topicName)

	// check the topic isn't already declared somewhere else
	for _, topic := range p.pubSubTopics {
		if strings.EqualFold(topic.Name, topicName) {
			p.errf(valueSpec.Pos(), "Pubsub topic names must be unique, \"%s\" was previously declared in %s/%s: if you wish to reuse the same topic, then you can export the original Topic object from %s and reuse it here.", topicName, topic.File.Pkg.Name, topic.File.Name, topic.File.Pkg.Name)
			return
		}
	}

	typeArgs := getTypeArguments(callExpr.Fun)
	if len(typeArgs) != 1 {
		p.errf(callExpr.Pos(), "pubsub.NewTopic requires exactly one type argument, the message payload type. For example `pubsub.NewTopic[MyMessage]((\"my-topic\")`")
		return
	}

	messageType := p.resolveParameter("pubsub message type", file.Pkg, file, typeArgs[0])

	// Record the topic
	topic := &est.PubSubTopic{
		Name:              topicName,
		Doc:               doc,
		DeliveryGuarantee: est.AtLeastOnce,
		Ordered:           false,
		GroupingField:     "",
		File:              file,
		MessageType:       messageType,
		AST:               valueSpec,
		Subscribers:       nil,
		Publishers:        nil,
	}
	p.pubSubTopics = append(p.pubSubTopics, topic)

	// Record the reference to the topic declaration
	file.References[valueSpec.Names[0]] = &est.Node{
		Type:  est.PubSubTopicDefNode,
		Topic: topic,
	}
}

func normaliseTopicName(name string) string {
	return identifiers.ConvertIdentifierTo(name, identifiers.KebabCase)
}
