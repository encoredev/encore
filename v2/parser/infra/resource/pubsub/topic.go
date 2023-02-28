package pubsub

import (
	"go/ast"

	"encr.dev/v2/internal/pkginfo"
	"encr.dev/v2/internal/schema"
	"encr.dev/v2/internal/schema/schemautil"
	literals2 "encr.dev/v2/parser/infra/internal/literals"
	"encr.dev/v2/parser/infra/internal/locations"
	parseutil2 "encr.dev/v2/parser/infra/internal/parseutil"
	"encr.dev/v2/parser/infra/resource"
)

type DeliveryGuarantee int

const (
	AtLeastOnce DeliveryGuarantee = iota
	ExactlyOnce
)

type Topic struct {
	File              *pkginfo.File
	Name              string              // The unique name of the pub sub topic
	Doc               string              // The documentation on the pub sub topic
	DeliveryGuarantee DeliveryGuarantee   // What guarantees does the pub sub topic have?
	OrderingKey       string              // What field in the message type should be used to ensure First-In-First-Out (FIFO) for messages with the same key
	MessageType       *schema.TypeDeclRef // The message type of the pub sub topic
}

func (t *Topic) Kind() resource.Kind       { return resource.PubSubTopic }
func (t *Topic) DeclaredIn() *pkginfo.File { return t.File }

var TopicParser = &resource.Parser{
	Name:      "PubSub Topic",
	DependsOn: nil,

	RequiredImports: []string{"encore.dev/pubsub"},
	Run: func(p *resource.Pass) []resource.Resource {
		name := pkginfo.QualifiedName{Name: "NewTopic", PkgPath: "encore.dev/pubsub"}

		spec := &parseutil2.ResourceCreationSpec{
			AllowedLocs: locations.AllowedIn(locations.Variable).ButNotIn(locations.Function, locations.FuncCall),
			MinTypeArgs: 1,
			MaxTypeArgs: 1,
			Parse:       parsePubSubTopic,
		}

		var resources []resource.Resource
		parseutil2.FindPkgNameRefs(p.Pkg, []pkginfo.QualifiedName{name}, func(file *pkginfo.File, name pkginfo.QualifiedName, stack []ast.Node) {
			r := parseutil2.ParseResourceCreation(p, spec, parseutil2.ReferenceData{
				File:         file,
				Stack:        stack,
				ResourceFunc: name,
			})
			if r != nil {
				resources = append(resources, r)
			}
		})
		return resources
	},
}

func parsePubSubTopic(d parseutil2.ParseData) resource.Resource {
	if len(d.Call.Args) != 2 {
		d.Pass.Errs.Add(d.Call.Pos(), "pubsub.NewTopic expects 2 arguments")
		return nil
	}

	topicName := parseutil2.ParseResourceName(d.Pass.Errs, "pubsub.NewTopic", "topic name",
		d.Call.Args[0], parseutil2.KebabName, "")
	if topicName == "" {
		// we already reported the error inside ParseResourceName
		return nil
	}

	messageType, ok := schemautil.ResolveNamedStruct(d.TypeArgs[0], false)
	if !ok {
		d.Pass.Errs.Add(d.Call.Pos(), "pubsub.NewTopic message type expects a named struct type as its type argument")
		return nil
	}

	cfgLit, ok := literals2.ParseStruct(d.Pass.Errs, d.File, "pubsub.TopicConfig", d.Call.Args[1])
	if !ok {
		return nil // error reported by ParseStruct
	}

	// Decode the config
	type decodedConfig struct {
		DeliveryGuarantee int    `literal:",required"`
		OrderingKey       string `literal:",optional"`
	}
	config := literals2.Decode[decodedConfig](d.Pass.Errs, cfgLit)

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
		File:              d.File,
		Name:              topicName,
		Doc:               d.Doc,
		DeliveryGuarantee: AtLeastOnce,
		OrderingKey:       config.OrderingKey,
		MessageType:       messageType,
	}
}
