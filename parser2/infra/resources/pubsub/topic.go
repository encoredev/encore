package pubsub

import (
	"go/ast"

	"encr.dev/parser2/infra/internal/literals"
	"encr.dev/parser2/infra/internal/locations"
	"encr.dev/parser2/infra/internal/parseutil"
	"encr.dev/parser2/infra/resources"
	"encr.dev/parser2/internal/pkginfo"
	"encr.dev/parser2/internal/schema"
	"encr.dev/parser2/internal/schema/schemautil"
)

type DeliveryGuarantee int

const (
	AtLeastOnce DeliveryGuarantee = iota
	ExactlyOnce
)

type Topic struct {
	Name              string              // The unique name of the pub sub topic
	Doc               string              // The documentation on the pub sub topic
	DeliveryGuarantee DeliveryGuarantee   // What guarantees does the pub sub topic have?
	OrderingKey       string              // What field in the message type should be used to ensure First-In-First-Out (FIFO) for messages with the same key
	MessageType       *schema.TypeDeclRef // The message type of the pub sub topic
}

func (t *Topic) Kind() resources.Kind { return resources.PubSubTopic }

var TopicParser = &resources.Parser{
	Name:      "PubSub Topic",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/pubsub"},
	Run: func(p *resources.Pass) {
		name := pkginfo.QualifiedName{Name: "NewTopic", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parsePubSubTopic,
		}

		parseutil.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			parseutil.ParseResourceCreation(p, spec, parseutil.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
		})
	},
}

func parsePubSubTopic(d parseutil.ParseData) resources.Resource {
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Add(d.Call.Pos(), "pubsub.NewTopic expects 2 arguments")
		return nil
	}

	topicName := parseutil.ParseResourceName(d.Pass.Errs, "pubsub.NewTopic", "topic name",
		d.Call.Args[0], parseutil.KebabName, "")
	if topicName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	messageType, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		d.Pass.Errs.Add(d.Call.Pos(), "pubsub.NewTopic message type expects a named struct type as its type argument")
		return nil
	}

	cfgLit, ok := literals.ParseStruct(d.Pass.Errs, d.File, "pubsub.TopicConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		DeliveryGuarantee int    `literal:",required"`
		OrderingKey       string `literal:",optional"`
	}
	config := literals.Decode[decodedConfig](d.Pass.Errs, cfgLit)

	// Get the ordering key
	if config.OrderingKey != "" {
		// Make sure the OrderingKey value exists in the struct.
		if str, ok := messageType.Decl.Type.(schema.StructType); ok {
			found := false
			for _, field := range str.Fields {
				if field.Name.Value == config.OrderingKey {
					found = true
					break
				}
			}

			if !found || !ast.IsExported(config.OrderingKey) {
				//p.errInSrc(srcerrors.PubSubOrderingKeyMustBeExported(p.fset, cfgLit.Expr("OrderingKey")))
				d.Pass.Errs.Addf(cfgLit.Pos("OrderingKey"), "Ordering Key must refer to an exported field in the message type")
			}
		}
	}

	return &Topic{
		Name:              topicName,
		Doc:               d.Doc,
		DeliveryGuarantee: AtLeastOnce,
		OrderingKey:       config.OrderingKey,
		MessageType:       messageType,
	}
}
